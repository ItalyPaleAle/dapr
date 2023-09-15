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
	"fmt"
	"net"

	"google.golang.org/grpc"

	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/dapr/pkg/security"
	"github.com/dapr/kit/logger"
)

var log = logger.NewLogger("dapr.actorssvc.server")

type Options struct {
	Port int

	Security security.Handler
}

// server is the gRPC server for the Actors service.
type server struct {
}

// Start starts the server. Blocks until the context is cancelled.
func Start(ctx context.Context, opts Options) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", opts.Port))
	if err != nil {
		return fmt.Errorf("could not listen on port %d: %w", opts.Port, err)
	}

	srv := grpc.NewServer(opts.Security.GRPCServerOptionNoClientAuth())
	s := &server{}
	actorsv1pb.RegisterActorsServer(srv, s)

	errCh := make(chan error, 1)
	go func() {
		log.Infof("Running gRPC server on port %d", opts.Port)
		if err := srv.Serve(lis); err != nil {
			errCh <- fmt.Errorf("failed to serve: %w", err)
			return
		}
		errCh <- nil
	}()

	<-ctx.Done()
	log.Info("Shutting down gRPC server")
	srv.GracefulStop()
	return <-errCh
}

// ConnectHost is used by the Dapr sidecar to register itself as an actor host.
// It remains active as a long-lived bi-di stream to allow for the Actors service
// to communicate with the sidecar.
func (s *server) ConnectHost(actorsv1pb.Actors_ConnectHostServer) error {
	panic("unimplemented")
}

// ReminderCompleted is used by the sidecar to acknowledge that a reminder has been executed successfully.
func (s *server) ReminderCompleted(context.Context, *actorsv1pb.ReminderCompletedRequest) (*actorsv1pb.ReminderCompletedResponse, error) {
	panic("unimplemented")
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

// CreateReminder creates a new reminder.
// If a reminder with the same ID (actor type, actor ID, name) already exists, it's replaced.
func (s *server) CreateReminder(context.Context, *actorsv1pb.CreateReminderRequest) (*actorsv1pb.CreateReminderResponse, error) {
	panic("unimplemented")
}

// GetReminder returns details about an existing reminder.
func (s *server) GetReminder(context.Context, *actorsv1pb.GetReminderRequest) (*actorsv1pb.GetReminderResponse, error) {
	panic("unimplemented")
}

// DeleteReminder removes an existing reminder before it fires.
func (s *server) DeleteReminder(context.Context, *actorsv1pb.DeleteReminderRequest) (*actorsv1pb.DeleteReminderResponse, error) {
	panic("unimplemented")
}
