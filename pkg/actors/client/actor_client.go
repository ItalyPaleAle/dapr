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

//nolint:protogetter
package client

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpcRetry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	kclock "k8s.io/utils/clock"

	"github.com/dapr/dapr/pkg/actors/internal"
	diag "github.com/dapr/dapr/pkg/diagnostics"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/dapr/pkg/resiliency"
	"github.com/dapr/dapr/pkg/security"
	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/ttlcache"
)

var log = logger.NewLogger("dapr.runtime.actors.client")

const dialTimeout = 20 * time.Second

var (
	_ internal.PlacementService  = (*ActorClient)(nil)
	_ internal.RemindersProvider = (*ActorClient)(nil)
)

// ActorClient is a client for the Actors service.
// It manages the placement of actors in the cluster as well as offers reminders services.
type ActorClient struct {
	actorsClient       actorsv1pb.ActorsClient
	conn               *grpc.ClientConn
	security           security.Handler
	lock               sync.Mutex
	executeReminderFn  internal.ExecuteReminderFn
	haltActorFn        internal.HaltActorFn
	haltAllActorsFn    internal.HaltAllActorsFn
	config             internal.Config
	actorTypes         []*actorsv1pb.ActorHostType
	addActorTypeCh     chan struct{}
	appHealthFn        internal.AppHealthFn
	appHealthCh        <-chan bool
	onAPILevelUpdate   func(apiLevel uint32)
	resiliency         resiliency.Provider
	running            atomic.Bool
	runningCtx         context.Context
	runningCancel      context.CancelFunc
	connectHostRunning atomic.Bool
	cache              *ttlcache.Cache[*actorsv1pb.LookupActorResponse]
	clock              kclock.Clock
	wg                 sync.WaitGroup
}

// NewActorClient initializes a new ActorClient object.
func NewActorClient(opts internal.ActorsProviderOptions) *ActorClient {
	// We do not init addActorTypeCh here
	return &ActorClient{
		security:    opts.Security,
		resiliency:  opts.Resiliency,
		appHealthFn: opts.AppHealthFn,
		config:      opts.Config,
		clock:       opts.Clock,
		actorTypes:  make([]*actorsv1pb.ActorHostType, 0),
	}
}

func (a *ActorClient) SetExecuteReminderFn(fn internal.ExecuteReminderFn) {
	a.executeReminderFn = fn
}

func (a *ActorClient) SetOnTableUpdateFn(fn func()) {
	// No-op in this implementation
}

func (a *ActorClient) SetHaltActorFns(haltFn internal.HaltActorFn, haltAllFn internal.HaltAllActorsFn) {
	a.haltActorFn = haltFn
	a.haltAllActorsFn = haltAllFn
}

func (a *ActorClient) SetOnAPILevelUpdate(fn func(apiLevel uint32)) {
	a.onAPILevelUpdate = fn
}

func (a *ActorClient) Init(ctx context.Context) error {
	// No-op in this implementation
	return nil
}

func (a *ActorClient) PlacementHealthy() bool {
	return a.connectHostRunning.Load()
}

func (a *ActorClient) StatusMessage() string {
	if a.connectHostRunning.Load() {
		return "actors client: connected"
	}
	return "actors client: disconnected"
}

// Start the service.
// If there's any hosted actor type, establishes the ConnectHost stream with the actors service.
func (a *ActorClient) Start(ctx context.Context) error {
	if !a.running.CompareAndSwap(false, true) {
		return errors.New("already started")
	}

	a.runningCtx, a.runningCancel = context.WithCancel(ctx)

	// Init the cache
	// This has a max TTL of 5s
	a.cache = ttlcache.NewCache[*actorsv1pb.LookupActorResponse](ttlcache.CacheOptions{
		MaxTTL: 5,
	})

	// Start checking app's health
	a.appHealthCh = a.appHealthFn(ctx)

	// Establish the connection with the Actors service
	a.establishGrpcConnection(ctx)

	// If we have actor types registered, start the ConnectHost stream right away
	a.lock.Lock()
	if len(a.actorTypes) > 0 {
		a.startConnectHost()
	}
	a.lock.Unlock()

	// Block until context is canceled
	<-a.runningCtx.Done()

	return nil
}

// Establishes a connection to the gRPC endpoint of the Actors service and creates a new client
func (a *ActorClient) establishGrpcConnection(ctx context.Context) error {
	var unaryClientInterceptor grpc.UnaryClientInterceptor
	if diag.DefaultGRPCMonitoring.IsEnabled() {
		unaryClientInterceptor = grpcMiddleware.ChainUnaryClient(
			grpcRetry.UnaryClientInterceptor(),
			diag.DefaultGRPCMonitoring.UnaryClientInterceptor(),
		)
	} else {
		unaryClientInterceptor = grpcRetry.UnaryClientInterceptor()
	}

	actorsID, err := spiffeid.FromSegments(a.security.ControlPlaneTrustDomain(), "ns", a.security.ControlPlaneNamespace(), "dapr-actors")
	if err != nil {
		return err
	}

	// Dial the gRPC connection
	addr := strings.TrimPrefix(a.config.ActorsService, "actors:")
	log.Debugf("Establishing connection with Actors service at address %s…", addr)
	ctx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()
	a.conn, err = grpc.DialContext(ctx, addr,
		grpc.WithUnaryInterceptor(unaryClientInterceptor),
		a.security.GRPCDialOptionMTLS(actorsID),
		grpc.WithReturnConnectionError(),
	)
	if err != nil {
		return err
	}

	// Create the client
	a.actorsClient = actorsv1pb.NewActorsClient(a.conn)

	return nil
}

// Starts the ConnectHost stream and sends the current list of actor types.
// It must be invoked by a caller that owns the lock.
func (a *ActorClient) startConnectHost() {
	go func() {
		// If there's no appHealthCh, something that happens when there's no app channel, we must assume that the app is always healthy
		if a.appHealthCh == nil {
			a.startConnectHostHealthy()
		}

		for {
			select {
			case healthy, ok := <-a.appHealthCh:
				// If the second value is false, it means the channel is closed
				// That happens when the context is canceled
				if !ok || a.runningCtx.Err() != nil {
					return
				}
				if healthy {
					// If we got a healthy signal, start the ConnectHost stream
					// Until this method returns, signals in the appHealthCh channel will be handled by the loop within this method
					a.startConnectHostHealthy()
				}
			case <-a.runningCtx.Done():
				return
			}
		}
	}()

	a.addActorTypeCh = make(chan struct{}, 1)
}

func (a *ActorClient) startConnectHostHealthy() {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0 // Retry forever
	bo.InitialInterval = 500 * time.Millisecond
	bo.MaxInterval = 5 * time.Second

	for {
		select {
		case <-a.runningCtx.Done():
			return
		default:
			// Check again to make sure the context isn't canceled at the same time
			if a.runningCtx.Err() != nil {
				return
			}

			err := a.establishConnectHost(a.actorTypes, bo)
			if err != nil {
				log.Errorf("Error from ConnectHost: %v", err)
			}
		}

		// Before reconnecting, use a backoff
		if a.runningCtx.Err() != nil {
			return
		}
		delay := bo.NextBackOff()
		log.Infof("Will attempt re-establishing ConnectHost in %v", delay)
		select {
		case <-time.After(delay):
			// Nop - can reconnect
		case <-a.runningCtx.Done():
			return
		}
	}
}

func (a *ActorClient) establishConnectHost(actorTypes []*actorsv1pb.ActorHostType, bo backoff.BackOff) error {
	ctx, cancel := context.WithCancel(a.runningCtx)
	defer cancel()

	// Establish the stream connection
	log.Debugf("Establishing ConnectHost stream with Actors service…")
	stream, err := a.actorsClient.ConnectHost(ctx)
	if err != nil {
		return fmt.Errorf("failed to establish ConnectHost stream: %w", err)
	}

	// Perform the handshake and receive the configuration
	config, err := a.connectHostHandshake(stream, actorTypes)
	if err != nil {
		return fmt.Errorf("handshake error: %w", err)
	}
	if a.onAPILevelUpdate != nil {
		a.onAPILevelUpdate(config.GetClusterApiLevel())
	}

	a.connectHostRunning.Store(true)

	// After a successful connection, reset the backoff
	bo.Reset()

	// When we disconnect from ConnectHost, we need to deactivate all actors
	defer func() {
		log.Debugf("Disconnected from ConnectHost stream with Actors service")

		// Also reset the lookup cache, as this indicates a likely failure of the actor placement subsystem
		a.cache.Reset()

		a.connectHostRunning.Store(false)

		if a.haltAllActorsFn != nil {
			haltErr := a.haltAllActorsFn()
			if haltErr != nil {
				log.Errorf("Failed to deactivate all actors: %v", haltErr)
			}
		}
	}()

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
			cs, rErr := stream.Recv()
			if rErr != nil {
				errCh <- fmt.Errorf("error from server: %w", rErr)
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
				rErr = msg.ActorHostConfiguration.Validate()
				if rErr != nil {
					errCh <- status.Errorf(codes.InvalidArgument, "Invalid request: %v", rErr)
					return
				}
				log.Debugf("Received actor configuration update")
				msgCh <- msg.ActorHostConfiguration

			case *actorsv1pb.ConnectHostServerStream_ExecuteReminder:
				// Received a reminder to execute
				if msg.ExecuteReminder.GetReminder() == nil {
					errCh <- errors.New("reminder message has nil Reminder inside")
					return
				}
				msgCh <- msg.ExecuteReminder

			case *actorsv1pb.ConnectHostServerStream_DeactivateActor:
				// Received a request to deactivate an actor
				if msg.DeactivateActor.GetActor() == nil {
					errCh <- errors.New("message to deactivate actor has nil Actor inside")
				}
				msgCh <- msg.DeactivateActor

			default:
				// Assume all other messages are a "pong" (response to ping)
				// TODO: Remove this debug log
				log.Debugf("Received pong from actors service")
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
				if msg.ClusterApiLevel != config.ClusterApiLevel && a.onAPILevelUpdate != nil {
					a.onAPILevelUpdate(msg.GetClusterApiLevel())
				}
				config = msg
				pingTicker.Reset(config.GetPingInterval())

			case *actorsv1pb.ExecuteReminder:
				// Executing the remidner
				if a.executeReminderFn != nil {
					a.wg.Add(1)
					go a.doExecuteReminder(ctx, msg)
				}

			case *actorsv1pb.DeactivateActor:
				// Deactivate an actor
				if a.haltActorFn != nil {
					err = a.haltActorFn(msg.Actor.ActorType, msg.Actor.ActorId)
					if err != nil {
						return fmt.Errorf("error while halting actor: %w", err)
					}
				}
			}

		case <-a.addActorTypeCh:
			// We need to communicate to the actors service that there's new actor types we support
			// First, get the list of actor types, which requires getting a lock
			// By the time we get a lock it's possible that the list of actor types has been modified since we read from the channel, but this should not be a problem because the list is append-only. In the worst case scenario, we'll get a second message on `addActorTypeCh` and we'll re-submit the (same) list again shortly.
			a.lock.Lock()
			actorTypes := a.actorTypes
			a.lock.Unlock()

			if log.IsOutputLevelEnabled(logger.DebugLevel) {
				actorTypeNames := make([]string, len(actorTypes))
				for i, at := range actorTypes {
					if at != nil {
						actorTypeNames[i] = at.GetActorType()
					}
					if actorTypeNames[i] == "" {
						actorTypeNames[i] = "(nil)"
					}
				}
				log.Debugf("Updating list of supported actor types with Actors service: %v", actorTypeNames)
			}

			err = stream.Send(&actorsv1pb.ConnectHostClientStream{
				Message: &actorsv1pb.ConnectHostClientStream_RegisterActorHost{
					RegisterActorHost: &actorsv1pb.RegisterActorHost{
						ActorTypes: actorTypes,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("error while updating the list of supported actor types: %w", err)
			}

		case healthy, ok := <-a.appHealthCh:
			// If the second value is false, it means the channel is closed
			// That happens when the context is canceled, in which case the service is shutting down
			if !ok {
				return nil
			}

			// If the app is reported as unhealthy, we need to disconnect
			if !healthy {
				return errors.New("app is unhealthy")
			}

		case <-stream.Context().Done():
			// When the context is done, it usually means that the service is shutting down
			// Let's just return
			log.Warn("ConnectHost stream ended with stream context done")
			return nil

		case <-ctx.Done():
			// Context passed to Start is being closed - the actor client is shutting down
			// Let's just return
			return nil
		}
	}
}

func (a *ActorClient) connectHostHandshake(stream actorsv1pb.Actors_ConnectHostClient, actorTypes []*actorsv1pb.ActorHostType) (*actorsv1pb.ActorHostConfiguration, error) {
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

	if log.IsOutputLevelEnabled(logger.DebugLevel) {
		actorTypeNames := make([]string, len(actorTypes))
		for i, at := range actorTypes {
			if at != nil {
				actorTypeNames[i] = at.GetActorType()
			}
			if actorTypeNames[i] == "" {
				actorTypeNames[i] = "(nil)"
			}
		}
		log.Debugf("Sending list of supported actor types to Actors service: %v", actorTypeNames)
	}

	// Send the first message to register this actor host
	err := stream.Send(&actorsv1pb.ConnectHostClientStream{
		Message: &actorsv1pb.ConnectHostClientStream_RegisterActorHost{
			RegisterActorHost: &actorsv1pb.RegisterActorHost{
				Address:    a.config.GetRuntimeHostname(),
				AppId:      a.config.AppID,
				ApiLevel:   internal.ActorAPILevel,
				ActorTypes: actorTypes,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send first message on ConnectHost stream: %w", err)
	}

	// Expect a response within 3s
	ctx, cancel := context.WithTimeout(a.runningCtx, 3*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timed out waiting for a response from ConnectHost registration message: %w", ctx.Err())
	case err = <-errCh:
		return nil, fmt.Errorf("error from actors service: %w", err)
	case msg := <-msgCh:
		// Ensure required fields are present
		err = msg.Validate()
		if err != nil {
			return nil, fmt.Errorf("invalid ActorHostConfiguration: %w", err)
		}
		return msg, nil
	}
}

// This method, which should be executed on a goroutine, executes a reminder
// If needed, stops the repeating reminder at the end.
func (a *ActorClient) doExecuteReminder(ctx context.Context, msg *actorsv1pb.ExecuteReminder) {
	ok := a.executeReminderFn(internal.NewReminderFromProto(msg.Reminder))

	// Send a message that the reminder was completed
	_, err := a.actorsClient.CompleteReminder(ctx, &actorsv1pb.CompleteReminderRequest{
		Ref:             msg.Reminder.GetRef(),
		CompletionToken: msg.CompletionToken,
		// If the method returns false, we need to stop processing the reminder and delete it
		// (If the reminder doesn't repeat, this has no effect anyways)
		StopReminder: !ok,
	})
	if err != nil {
		log.Errorf("Failed to complete reminder: %v", err)
	}
}

func (a *ActorClient) CreateReminder(ctx context.Context, reminder *internal.Reminder) error {
	_, err := a.actorsClient.CreateReminder(ctx, &actorsv1pb.CreateReminderRequest{
		Reminder: reminder.ToProto(),
	})
	if err != nil {
		return fmt.Errorf("error from actors service: %w", err)
	}
	return nil
}

func (a *ActorClient) GetReminder(ctx context.Context, req *internal.GetReminderRequest) (*internal.Reminder, error) {
	res, err := a.actorsClient.GetReminder(ctx, &actorsv1pb.GetReminderRequest{
		Ref: req.ToRefProto(),
	})
	if err != nil {
		return nil, fmt.Errorf("error from actors service: %w", err)
	}

	reminder := res.GetReminder()
	if reminder == nil {
		return nil, errors.New("reminder is nil in response")
	}

	return internal.NewReminderFromProto(res.Reminder), nil
}

func (a *ActorClient) DeleteReminder(ctx context.Context, req internal.DeleteReminderRequest) error {
	_, err := a.actorsClient.DeleteReminder(ctx, &actorsv1pb.DeleteReminderRequest{
		Ref: req.ToRefProto(),
	})
	if err != nil {
		return fmt.Errorf("error from actors service: %w", err)
	}
	return nil
}

// AddHostedActorType registers an actor type by adding it to the list of known actor types (if it's not already registered).
func (a *ActorClient) AddHostedActorType(actorType string, idleTimeout time.Duration) error {
	// We need a lock here
	a.lock.Lock()
	defer a.lock.Unlock()

	for _, t := range a.actorTypes {
		if t.ActorType == actorType {
			return fmt.Errorf("actor type %s already registered", actorType)
		}
	}

	// Add the actor type to the slice
	proto := &actorsv1pb.ActorHostType{
		ActorType:   actorType,
		IdleTimeout: uint32(idleTimeout.Seconds()),
	}
	a.actorTypes = append(a.actorTypes, proto)

	// If the service hasn't started yet, just return
	if !a.running.Load() {
		return nil
	}

	// Check if we are already connected to ConnectHost
	if a.addActorTypeCh == nil {
		// Need to connect
		a.startConnectHost()
	} else {
		// We are already connected so just send to the channel that there's an update to the actor types we support
		// This is a buffered channel with capacity of 1, which allows us to batch changes
		a.addActorTypeCh <- struct{}{}
	}
	return nil
}

// Closes shuts down server stream gracefully.
func (a *ActorClient) Close() error {
	if !a.running.CompareAndSwap(true, false) {
		return nil
	}

	var errs []error
	errs = append(errs, a.conn.Close())
	a.cache.Stop()
	a.runningCancel()

	a.wg.Wait()

	return errors.Join(errs...)
}

func (a *ActorClient) WaitUntilReady(ctx context.Context) error {
	// WaitUntilReady is a no-op in this implementation.
	return nil
}

func (a *ActorClient) LookupActor(ctx context.Context, req internal.LookupActorRequest) (res internal.LookupActorResponse, err error) {
	cacheKey := req.ActorKey()

	// Get the value from the cache if possible
	var lar *actorsv1pb.LookupActorResponse
	if !req.NoCache {
		var ok bool
		lar, ok = a.cache.Get(cacheKey)
		if !ok {
			lar = nil
		}
	}

	// If thre's no cached value, fetch it from the server
	if lar == nil {
		lar, err = a.actorsClient.LookupActor(ctx, &actorsv1pb.LookupActorRequest{
			Actor:   createActorRef(req.ActorType, req.ActorID),
			NoCache: req.NoCache,
		})
		if err != nil {
			if status.Code(err) == codes.FailedPrecondition {
				return res, fmt.Errorf("did not find address for actor %s/%s", req.ActorType, req.ActorID)
			}
			return res, fmt.Errorf("error from Actors service: %w", err)
		}

		// Store in the cache
		a.cache.Set(cacheKey, lar, int64(lar.IdleTimeout))
	}

	// Return
	res.Address = lar.Address
	res.AppID = lar.AppId
	return res, nil
}

// ReportActorDeactivation notifies the Actors service that an actor has been deactivated.
func (a *ActorClient) ReportActorDeactivation(ctx context.Context, actorType, actorID string) error {
	// If ConnectHost isn't running, this instance doesn't have any active actor already
	if !a.connectHostRunning.Load() {
		return nil
	}

	_, err := a.actorsClient.ReportActorDeactivation(ctx, &actorsv1pb.ReportActorDeactivationRequest{
		Actor: createActorRef(actorType, actorID),
	})
	if err != nil {
		// If the error is a NotFound, it means the actor was already inactive.
		// That's an error we should log, but we shouldn't return that as error
		if status.Code(err) == codes.NotFound {
			log.Warnf("Attempted to deactivate actor '%s/%s' that was already inactive in the Actors service: %v", actorType, actorID, err)
			return nil
		}
		return fmt.Errorf("error from Actors service: %w", err)
	}
	return nil
}

func createActorRef(actorType, actorID string) *actorsv1pb.ActorRef {
	return &actorsv1pb.ActorRef{
		ActorType: actorType,
		ActorId:   actorID,
	}
}

func (a *ActorClient) SetStateStoreProviderFn(fn internal.StateStoreProviderFn) {
	// SetStateStoreProviderFn is a no-op in this implementation.
}

func (a *ActorClient) SetLookupActorFn(fn internal.LookupActorFn) {
	// SetLookupActorFn is a no-op in this implementation.
}

func (a *ActorClient) OnPlacementTablesUpdated(ctx context.Context) {
	// OnPlacementTablesUpdated is a no-op in this implementation.
}

func (a *ActorClient) DrainRebalancedReminders(actorType string, actorID string) {
	// DrainRebalancedReminders is a no-op in this implementation.
}
