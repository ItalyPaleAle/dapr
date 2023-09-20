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

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapr/components-contrib/actorstore"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
)

// ConnectHost is used by the Dapr sidecar to register itself as an actor host.
// It remains active as a long-lived bi-di stream to allow for the Actors service
// to communicate with the sidecar.
func (s *server) ConnectHost(stream actorsv1pb.Actors_ConnectHostServer) error {
	// Receive the first message
	msg, err := s.connectHostReceiveFirstMessage(stream)
	if err != nil {
		// If the error is io.EOF, it means that the stream has ended
		if errors.Is(err, io.EOF) {
			return nil
		}
		log.Warnf("Error receiving first message in ConnectHost: %v", err)
		return err
	}

	// Register the actor host
	actorHostID, err := s.store.AddActorHost(stream.Context(), msg.ToActorStoreRequest())
	if err != nil {
		log.Errorf("Failed to register actor host: %v", err)
		return fmt.Errorf("failed to register actor host: %w", err)
	}
	log.Debugf("Registered actor host: id='%s' appID='%s' address='%s' actorTypes=%v", actorHostID, msg.AppId, msg.Address, msg.GetActorTypes())

	// Send the relevant configuration to the actor host
	err = stream.Send(s.opts.GetActorHostConfigurationMessage())
	if err != nil {
		log.Errorf("Failed to send configuration to actor host: %v", err)
		return fmt.Errorf("failed to send configuration to actor host: %w", err)
	}

	// Timeout for healthchecks
	healthCheckTimeout := time.NewTimer(s.opts.HostHealthCheckInterval)
	stopHealthCheckTimeout := func() {
		if !healthCheckTimeout.Stop() {
			select {
			case <-healthCheckTimeout.C:
			default:
			}
		}
	}
	defer stopHealthCheckTimeout()

	// Read messages in background
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
				errCh <- fmt.Errorf("error from client: %w", err)
				return
			}

			// Message is a "oneof" that can have multiple values
			switch msg := cs.GetMessage().(type) {
			case *actorsv1pb.ConnectHostClientStream_RegisterActorHost:
				// Received a registration update
				// Validate it before processing it
				if msg.RegisterActorHost == nil {
					errCh <- errors.New("registration update message has nil RegisterActorHost property")
					return
				}
				err = msg.RegisterActorHost.ValidateUpdateMessage()
				if err != nil {
					errCh <- status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
					return
				}
				log.Debugf("Received registration update from actor host id='%s'", actorHostID)
				msgCh <- msg.RegisterActorHost
			default:
				// Assume all other messages are a ping
				// TODO: Remove this debug log
				log.Debugf("Received ping from actor host id='%s'", actorHostID)
				msgCh <- nil
			}
		}
	}()

	unregisterOnClose := true
	defer func() {
		if !unregisterOnClose {
			return
		}

		// Use a background context here because the client may have already disconnected
		log.Debugf("Uregistering actor host '%s'", actorHostID)
		err = s.store.RemoveActorHost(context.Background(), actorHostID)
		if err != nil {
			// Ignore "ErrActorHostNotFound" errors because it may be due to a race condition with removing the host
			if errors.Is(err, actorstore.ErrActorHostNotFound) {
				log.Debugf("Tried to un-register actor host '%s' that was already removed: %v", actorHostID, err)
			} else {
				log.Errorf("Failed to un-register actor host '%s': %v", actorHostID, err)
			}
		}
	}()

	// Repeat while the stream is connected
	emptyResponse := &actorsv1pb.ConnectHostServerStream{}
	for {
		select {
		case msgAny := <-msgCh:
			// Any message counts as a ping (at the very least), so update the actor host table
			var req actorstore.UpdateActorHostRequest
			switch msg := msgAny.(type) {
			case *actorsv1pb.RegisterActorHost:
				// We received a request to update the actor host's registration
				// This also counts as a ping
				req = msg.ToUpdateActorHostRequest()
				req.UpdateLastHealthCheck = true
			default:
				// We received a ping
				req.UpdateLastHealthCheck = true
			}
			err = s.store.UpdateActorHost(stream.Context(), actorHostID, req)
			if err != nil {
				log.Errorf("Failed to update actor host after ping: %v", err)
				return fmt.Errorf("failed to update actor host after ping: %w", err)
			}

			// Send a "pong"
			// This allows us to test if the channel is up more reliably, as we'd get a failure in case of channel failures
			err = stream.Send(emptyResponse)
			if err != nil {
				log.Errorf("Failed to send ping response to actor host: %v", err)
				return fmt.Errorf("failed to send ping response to actor host: %w", err)
			}

			// Reset the healthcheck timer
			stopHealthCheckTimeout()
			healthCheckTimeout.Reset(s.opts.HostHealthCheckInterval)

		case <-healthCheckTimeout.C:
			// Host hasn't sent a ping in the required amount of time, so we must assume it's offline
			log.Warnf("Actor host '%s' hasn't sent a ping in %v and is assumed to be in a failed state", actorHostID, s.opts.HostHealthCheckInterval)
			return status.Errorf(codes.DeadlineExceeded, "Did not receive a ping in %v", s.opts.HostHealthCheckInterval)

		case <-stream.Context().Done():
			// Normally, context cancelation indicates that the server is shutting down
			// We consider this equivalent to the client disconnecting
			// In this case, however, we do not unregister the actor host when this method returns, because the host will likely reconnect shortly after to resume
			unregisterOnClose = false
			log.Debugf("Actor host '%s' has disconnected", actorHostID)
			return nil

		case <-s.shutdownCh:
			// When we get a message on shutdownCh, it also indicates that the server is shutting down
			// We treat this like the case above
			unregisterOnClose = false
			log.Debugf("Disconnecting from actor host '%s' because server is shutting down", actorHostID)
			return nil

		case err := <-errCh:
			// io.EOF or context canceled signifies the client has disconnected
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				log.Debugf("Actor host '%s' has disconnected", actorHostID)
				return nil
			}
			log.Warnf("Error in ConnectHost stream from actor host '%s': %v", actorHostID, err)
			return err
		}
	}
}

// Receives the first message sent on connectHost, with a timeout.
func (s *server) connectHostReceiveFirstMessage(stream actorsv1pb.Actors_ConnectHostServer) (*actorsv1pb.RegisterActorHost, error) {
	// To start, we expect callers to send a message of type RegisterActorHost
	// We give callers 5s before disconnecting them
	msgCh := make(chan *actorsv1pb.RegisterActorHost)
	errCh := make(chan error)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		cs, err := stream.Recv()
		if err != nil {
			errCh <- err
			return
		}

		msg := cs.GetRegisterActorHost()
		if msg == nil {
			// If msg is nil, it was not a RegisterActorHost message
			errCh <- status.Error(codes.InvalidArgument, "Received an unexpected first message from caller: expecting a RegisterActorHost")
			return
		}
		msgCh <- msg
	}()

	ctx, cancel := context.WithTimeout(stream.Context(), 5*time.Second)
	defer cancel()
	select {
	case msg := <-msgCh:
		// Ensure required fields are present
		err := msg.ValidateFirstMessage()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		return msg, nil
	case <-ctx.Done():
		// If the error is context canceled, it means the client disconnected, so we convert it to io.EOF (just like when the stream ends)
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, io.EOF
		}
		return nil, status.Error(codes.DeadlineExceeded, "Timed out while waiting for the first message")
	case err := <-errCh:
		return nil, fmt.Errorf("error while receiving first message: %w", err)
	}
}

// LookupActor returns the address of an actor.
// If the actor is not active yet, it returns the address of an actor host capable of hosting it.
func (s *server) LookupActor(ctx context.Context, req *actorsv1pb.LookupActorRequest) (*actorsv1pb.LookupActorResponse, error) {
	err := req.GetActor().Validate()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid actor reference: %v", err)
	}

	lar, err := s.store.LookupActor(ctx, req.Actor.ToInternalActorRef())
	if err != nil {
		if errors.Is(err, actorstore.ErrNoActorHost) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}

		log.Errorf("Failed to perform actor lookup: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to perform actor lookup: %v", err)
	}

	return &actorsv1pb.LookupActorResponse{
		AppId:       lar.AppID,
		Address:     lar.Address,
		IdleTimeout: lar.IdleTimeout,
	}, nil
}

// ReportActorDeactivation is sent to report an actor that has been deactivated.
func (s *server) ReportActorDeactivation(ctx context.Context, req *actorsv1pb.ReportActorDeactivationRequest) (*actorsv1pb.ReportActorDeactivationResponse, error) {
	err := req.GetActor().Validate()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid actor reference: %v", err)
	}

	err = s.store.RemoveActor(ctx, req.Actor.ToInternalActorRef())
	if err != nil {
		if errors.Is(err, actorstore.ErrActorNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		log.Errorf("Failed to remove actor from database: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to remove actor from database: %v", err)
	}

	return &actorsv1pb.ReportActorDeactivationResponse{}, nil
}
