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

	"github.com/dapr/components-contrib/actorstore"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/kit/logger"
)

var log = logger.NewLogger("dapr.actorssvc.server")

// server is the gRPC server for the Actors service.
type server struct {
	opts  Options
	store actorstore.Store
	srv   *grpc.Server
}

// Start starts the server. Blocks until the context is cancelled.
func Start(ctx context.Context, opts Options) error {
	// Init the server
	s := &server{}
	err := s.Init(ctx, opts)
	if err != nil {
		return fmt.Errorf("failed to init server: %w", err)
	}

	// Blocks until context is canceled
	return s.Run(ctx)
}

func (s *server) Init(ctx context.Context, opts Options) error {
	s.opts = opts

	// Create the gRPC server
	s.srv = grpc.NewServer(opts.Security.GRPCServerOptionNoClientAuth())
	actorsv1pb.RegisterActorsServer(s.srv, s)

	// Init the store
	err := s.initActorStore(ctx)
	if err != nil {
		return fmt.Errorf("failed to init actor store: %w", err)
	}

	return nil
}

func (s *server) initActorStore(ctx context.Context) (err error) {
	s.store, err = s.opts.GetActorStore()
	if err != nil {
		return err
	}

	err = s.store.Init(ctx, s.opts.GetActorStoreMetadata())
	if err != nil {
		return err
	}

	return nil
}

func (s *server) Run(ctx context.Context) error {
	defer func() {
		err := s.store.Close()
		if err != nil {
			log.Errorf("Error while closing actor store: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.opts.Port))
	if err != nil {
		return fmt.Errorf("could not listen on port %d: %w", s.opts.Port, err)
	}

	errCh := make(chan error, 1)
	go func() {
		log.Infof("Running gRPC server on port %d", s.opts.Port)
		if err := s.srv.Serve(lis); err != nil {
			errCh <- fmt.Errorf("failed to serve: %w", err)
			return
		}
		errCh <- nil
	}()

	<-ctx.Done()
	log.Info("Shutting down gRPC server")
	s.srv.GracefulStop()
	return <-errCh
}
