/*
Copyright 2023 The Dapr Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package placement

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapr/dapr/pkg/actors/internal"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/dapr/pkg/resiliency"
	"github.com/dapr/dapr/utils/actorscache"
	"github.com/dapr/kit/logger"
)

var log = logger.NewLogger("dapr.runtime.actors.placementv2")

// actorPlacement is used to interact with the actors service to manage the placement of actors in the cluster.
type actorPlacement struct {
	actorsClient   actorsv1pb.ActorsClient
	address        string
	appID          string
	lock           sync.Mutex
	actorTypes     []*actorsv1pb.ActorHostType
	addActorTypeCh chan struct{}
	appHealthCh    chan bool
	resiliency     resiliency.Provider
	running        atomic.Bool
	runningCtx     context.Context
	runningCancel  context.CancelFunc
	cache          *actorscache.Cache[*actorsv1pb.LookupActorResponse]
}

// ActorPlacementOpts contains options for NewActorPlacement.
type ActorPlacementOpts struct {
	ActorsClient actorsv1pb.ActorsClient
	Address      string
	AppID        string
	Resiliency   resiliency.Provider
	// TODO: Needs to be used
	AppHealthCh chan bool
}

// NewActorPlacement initializes ActorPlacement for the actor service.
func NewActorPlacement(opts ActorPlacementOpts) internal.PlacementService {
	// We do not init addActorTypeCh here
	return &actorPlacement{
		actorsClient: opts.ActorsClient,
		address:      opts.Address,
		appID:        opts.AppID,
		resiliency:   opts.Resiliency,
		actorTypes:   make([]*actorsv1pb.ActorHostType, 0),
	}
}

// AddHostedActorType registers an actor type by adding it to the list of known actor types (if it's not already registered).
func (p *actorPlacement) AddHostedActorType(actorType string, idleTimeout time.Duration) error {
	// We need a lock here
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, t := range p.actorTypes {
		if t.ActorType == actorType {
			return fmt.Errorf("actor type %s already registered", actorType)
		}
	}

	// Add the actor type to the slice
	proto := &actorsv1pb.ActorHostType{
		ActorType:   actorType,
		IdleTimeout: uint32(idleTimeout.Seconds()),
	}
	p.actorTypes = append(p.actorTypes, proto)

	// If the service hasn't started yet, just return
	if !p.running.Load() {
		return nil
	}

	// Check if we are already connected to ConnectHost
	if p.addActorTypeCh == nil {
		// Need to connect
		p.startConnectHost()
	} else {
		// We are already connected so just send to the channel that there's an update to the actor types we support
		// This is a buffered channel with capacity of 1, which allows us to batch changes
		p.addActorTypeCh <- struct{}{}
	}
	return nil
}

// Start the service.
// If there's any hosted actor type, establishes the ConnectHost stream with the actors service.
func (p *actorPlacement) Start(ctx context.Context) error {
	if !p.running.CompareAndSwap(false, true) {
		return errors.New("already started")
	}

	p.runningCtx, p.runningCancel = context.WithCancel(ctx)

	// Init the cache
	// This has a max TTL of 5s
	p.cache = actorscache.NewCache[*actorsv1pb.LookupActorResponse](actorscache.CacheOptions{
		MaxTTL: 5,
	})

	// If we have actor types registered, start the ConnectHost stream right away
	p.lock.Lock()
	if len(p.actorTypes) > 0 {
		p.startConnectHost()
	}
	p.lock.Unlock()

	// Block until context is canceled
	<-p.runningCtx.Done()

	return nil
}

// Starts the ConnectHost stream and sends the current list of actor types.
// It must be invoked by a caller that owns the lock.
func (p *actorPlacement) startConnectHost() {
	go func() {
		err := p.establishConnectHost(p.actorTypes)
		if err != nil {
			// TODO: Handle errors better and restart the connection
			log.Errorf("Error from ConnectHost: %v", err)
		}
	}()
	p.addActorTypeCh = make(chan struct{}, 1)
}

func (p *actorPlacement) establishConnectHost(actorTypes []*actorsv1pb.ActorHostType) error {
	// Establish the stream connection
	stream, err := p.actorsClient.ConnectHost(p.runningCtx)
	if err != nil {
		return fmt.Errorf("failed to establish ConnectHost stream: %w", err)
	}

	// Perform the handshake and receive the configuration
	config, err := p.connectHostHandshake(stream, actorTypes)
	if err != nil {
		return fmt.Errorf("handshake error: %w", err)
	}

	// Ticker for pings
	pingTicker := time.NewTicker(config.GetPingInterval())
	defer pingTicker.Stop()

	// Read messages from the actors service in background
	msgCh := make(chan interface{ ProtoMessage() })
	errCh := make(chan error)
	go func() {
		defer func() {
			close(msgCh)
			close(errCh)
		}()
		for {
			// Block until we receive a message
			// This returns an error when the stream ends or in case of errors
			cs, err := stream.Recv()
			if err != nil {
				errCh <- fmt.Errorf("error from server: %w", err)
				return
			}

			// Message is a "oneof" that can have multiple values
			switch msg := cs.GetMessage().(type) {
			case *actorsv1pb.ConnectHostServerStream_ActorHostConfiguration:
				// Received a configuration update
				// Validate it before processing it
				if msg.ActorHostConfiguration == nil {
					errCh <- errors.New("configuration update message has nil ActorHostConfiguration property")
					return
				}
				err = msg.ActorHostConfiguration.Validate()
				if err != nil {
					errCh <- status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
					return
				}
				log.Debugf("Received actor configuration update")
				msgCh <- msg.ActorHostConfiguration

			case *actorsv1pb.ConnectHostServerStream_ExecuteReminder:
				panic("TODO: Unimplemented")

			case *actorsv1pb.ConnectHostServerStream_DeactivateActor:
				panic("TODO: Unimplemented")

			default:
				// Assume all other messages are a "pong" (response to ping)
				// TODO: Remove this debug log
				log.Debugf("Received pong from actor service")
				msgCh <- nil
			}
		}
	}()

	// Process loop
	emptyMsg := &actorsv1pb.ConnectHostClientStream{}
	for {
		select {
		case <-pingTicker.C:
			// Send a ping message when there's a tick
			err = stream.Send(emptyMsg)
			if err != nil {
				return fmt.Errorf("ping error: %w", err)
			}

		case msgAny := <-msgCh:
			// We received a message
			switch msg := msgAny.(type) {
			case *actorsv1pb.ActorHostConfiguration:
				// Update the configuration and reset the ticker
				config = msg
				pingTicker.Reset(config.GetPingInterval())
			case *actorsv1pb.ExecuteReminder:
				panic("TODO: Unimplemented")
			case *actorsv1pb.DeactivateActor:
				panic("TODO: Unimplemented")
			}

		case <-p.addActorTypeCh:
			// We need to communicate to the actors service that there's new actor types we support
			// First, get the list of actor types, which requires getting a lock
			// By the time we get a lock it's possible that the list of actor types has been modified since we read from the channel, but this should not be a problem because the list is append-only. In the worst case scenario, we'll get a second message on `addActorTypeCh` and we'll re-submit the (same) list again shortly.
			p.lock.Lock()
			err = stream.Send(&actorsv1pb.ConnectHostClientStream{
				Message: &actorsv1pb.ConnectHostClientStream_RegisterActorHost{
					RegisterActorHost: &actorsv1pb.RegisterActorHost{
						ActorTypes: p.actorTypes,
					},
				},
			})
			p.lock.Unlock()
			if err != nil {
				return fmt.Errorf("error while updating the list of supported actor types: %w", err)
			}

		case <-stream.Context().Done():
			// When the context is done, it usually means that the service is shutting down
			// Let's just return
			return nil

		case <-p.runningCtx.Done():
			// Context passed to Start is being closed
			// Let's just return
			return nil
		}
	}
}

func (p *actorPlacement) connectHostHandshake(stream actorsv1pb.Actors_ConnectHostClient, actorTypes []*actorsv1pb.ActorHostType) (*actorsv1pb.ActorHostConfiguration, error) {
	// To start, we expect callers to send a message of type RegisterActorHost
	// We give callers 5s before disconnecting them
	msgCh := make(chan *actorsv1pb.ActorHostConfiguration)
	errCh := make(chan error)
	go func() {
		defer func() {
			close(msgCh)
			close(errCh)
		}()
		cs, err := stream.Recv()
		if err != nil {
			errCh <- err
			return
		}

		msg := cs.GetActorHostConfiguration()
		if msg == nil {
			// If msg is nil, it was not a RegisterActorHost message
			errCh <- status.Error(codes.InvalidArgument, "Received an unexpected first message from caller: expecting an ActorHostConfiguration")
			return
		}
		msgCh <- msg
	}()

	// Send the first message to register this actor host
	err := stream.Send(&actorsv1pb.ConnectHostClientStream{
		Message: &actorsv1pb.ConnectHostClientStream_RegisterActorHost{
			RegisterActorHost: &actorsv1pb.RegisterActorHost{
				Address:    p.address,
				AppId:      p.appID,
				ApiLevel:   internal.ActorAPILevel,
				ActorTypes: actorTypes,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send first message on ConnectHost stream: %w", err)
	}

	// Expect a response within 3s
	ctx, cancel := context.WithTimeout(p.runningCtx, 3*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for a response from ConnectHost registration message: %w", ctx.Err())
	case msg := <-msgCh:
		// Ensure required fields are present
		err = msg.Validate()
		if err != nil {
			return nil, fmt.Errorf("invalid ActorHostConfiguration: %w", err)
		}
		return msg, nil
	}
}

// Closes shuts down server stream gracefully.
func (p *actorPlacement) Close() error {
	if !p.running.CompareAndSwap(true, false) {
		return nil
	}
	p.cache.Stop()
	p.runningCancel()

	return nil
}

func (p *actorPlacement) WaitUntilReady(ctx context.Context) error {
	// WaitUntilReady is a no-op in this implementation.
	return nil
}

func (p *actorPlacement) LookupActor(ctx context.Context, req internal.LookupActorRequest) (res internal.LookupActorResponse, err error) {
	cacheKey := req.ActorKey()

	// Get the value from the cache if possible
	var lar *actorsv1pb.LookupActorResponse
	if !req.NoCache {
		var ok bool
		lar, ok = p.cache.Get(cacheKey)
		if !ok {
			lar = nil
		}
	}

	// If thre's no cached value, fetch it from the server
	if lar == nil {
		lar, err = p.actorsClient.LookupActor(ctx, &actorsv1pb.LookupActorRequest{
			Actor:   createActorRef(req.ActorType, req.ActorID),
			NoCache: req.NoCache,
		})
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				return res, fmt.Errorf("did not find address for actor %s/%s", req.ActorType, req.ActorID)
			}
			return res, fmt.Errorf("error from Actor service: %w", err)
		}

		// Store in the cache
		p.cache.Set(cacheKey, lar, int64(lar.IdleTimeout))
	}

	// Return
	res.Address = lar.Address
	res.AppID = lar.AppId
	return res, nil
}

func (p *actorPlacement) ReportActorDeactivation(ctx context.Context, actorType, actorID string) error {
	_, err := p.actorsClient.ReportActorDeactivation(ctx, &actorsv1pb.ReportActorDeactivationRequest{
		Actor: createActorRef(actorType, actorID),
	})
	if err != nil {
		return fmt.Errorf("error from Actor service: %w", err)
	}
	return nil
}

func createActorRef(actorType, actorID string) *actorsv1pb.ActorRef {
	return &actorsv1pb.ActorRef{
		ActorType: actorType,
		ActorId:   actorID,
	}
}
