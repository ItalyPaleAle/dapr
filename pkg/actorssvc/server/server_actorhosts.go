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
	log.Debugf("Registered actor host: id='%s' appID='%s' address='%s'", actorHostID, msg.AppId, msg.Address)

	// Read messages in background
	pingCh := make(chan struct{})
	msgRegisterActorHostCh := make(chan *actorsv1pb.RegisterActorHost)
	errCh := make(chan error)
	go func() {
		defer func() {
			close(pingCh)
			close(msgRegisterActorHostCh)
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
					errCh <- err
					return
				}
				log.Debugf("Received registration update from actor host id='%s'", actorHostID)
				msgRegisterActorHostCh <- msg.RegisterActorHost
			default:
				// Assume all other messages are a ping
				// TODO: Remove this debug log
				log.Debugf("Received ping from actor host id='%s'", actorHostID)
				pingCh <- struct{}{}
			}
		}
	}()

	defer func() {
		// Use a background context here because the client may have already disconnected
		log.Debugf("Uregistering actor host '%s'", actorHostID)
		err = s.store.RemoveActorHost(context.Background(), actorHostID)
		if err != nil {
			log.Errorf("Failed to un-register actor host: %v", err)
		}
	}()

	select {
	case <-stream.Context().Done():
		log.Debugf("Actor host '%s' has disconnected", actorHostID)
		break
	case err := <-errCh:
		fmt.Println("ERR HERE", err)
		// io.EOF or context canceled signifies the client has disconnected
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}

	return nil
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
			errCh <- errors.New("received an unexpected first message from caller: expecting a RegisterActorHost")
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
			return nil, err
		}
		return msg, nil
	case <-ctx.Done():
		// If the error is context canceled, it means the client disconnected, so we convert it to io.EOF (just like when the stream ends)
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil, io.EOF
		}
		return nil, errors.New("timed out while waiting for the first message")
	case err := <-errCh:
		return nil, fmt.Errorf("error while receiving first message: %w", err)
	}
}

// LookupActor returns the address of an actor.
// If the actor is not active yet, it returns the address of an actor host capable of hosting it.
func (s *server) LookupActor(context.Context, *actorsv1pb.LookupActorRequest) (*actorsv1pb.LookupActorResponse, error) {
	panic("unimplemented")
}

// ReportActorDeactivation is sent to report an actor that has been deactivated.
func (s *server) ReportActorDeactivation(context.Context, *actorsv1pb.ReportActorDeactivationRequest) (*actorsv1pb.ReportActorDeactivationResponse, error) {
	panic("unimplemented")
}
