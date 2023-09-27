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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync"

	"google.golang.org/grpc"

	"github.com/dapr/components-contrib/actorstore"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/kit/logger"
)

var log = logger.NewLogger("dapr.actorssvc.server")

// server is the gRPC server for the Actors service.
type server struct {
	opts       Options
	store      actorstore.Store
	srv        *grpc.Server
	shutdownCh chan struct{}

	// "Process ID", which is generated randomly when the server is initialized.
	pid string

	// This map contains the list of active connections from actor hosts.
	// We use a "regular" map with a RWMutex instead of a sync.Map because we need to be sure that once we get a channel from the map, it's still valid when we attempt to use it.
	connectedHosts           connectedHosts
	connectedHostsIDs        []string
	connectedHostsActorTypes []string
	connectedHostsLock       sync.RWMutex
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

func (s *server) Init(ctx context.Context, opts Options) (err error) {
	s.opts = opts
	s.shutdownCh = make(chan struct{})
	s.connectedHosts = make(connectedHosts)

	// Generate a random PID
	s.pid, err = generatePID()
	if err != nil {
		return fmt.Errorf("failed to generate random process ID: %w", err)
	}

	log.Infof("Actors subsystem configuration: %v", s.opts.GetActorsConfiguration())

	// Create the gRPC server
	s.srv = grpc.NewServer(opts.Security.GRPCServerOptionMTLS())
	actorsv1pb.RegisterActorsServer(s.srv, s)

	// Init the store
	err = s.initActorStore(ctx)
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

	err = s.store.Init(ctx, s.opts.GetActorStoreMetadata(s.pid))
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

	gracefulShutdownCh := make(chan struct{})
	go func() {
		s.srv.GracefulStop()
		close(gracefulShutdownCh)
	}()
	close(s.shutdownCh)
	<-gracefulShutdownCh

	return <-errCh
}

func (s *server) ServiceInfo(ctx context.Context, req *actorsv1pb.ServiceInfoRequest) (*actorsv1pb.ServiceInfoResponse, error) {
	return &actorsv1pb.ServiceInfoResponse{
		Version: ActorsServiceVersion,
	}, nil
}

// Adds or updates a connected host.
func (s *server) setConnectedHost(actorHostID string, info connectedHostInfo) {
	// Set in the connectedHosts map then update the cached actor types
	s.connectedHostsLock.Lock()
	s.connectedHosts[actorHostID] = info
	s.connectedHostsIDs, s.connectedHostsActorTypes = s.connectedHosts.updateCachedData(s.connectedHostsIDs, s.connectedHostsActorTypes)
	s.connectedHostsLock.Unlock()
}

// Removes a connected host
func (s *server) removeConnectedHost(actorHostID string) {
	s.connectedHostsLock.Lock()
	delete(s.connectedHosts, actorHostID)
	s.connectedHostsIDs, s.connectedHostsActorTypes = s.connectedHosts.updateCachedData(s.connectedHostsIDs, s.connectedHostsActorTypes)
	s.connectedHostsLock.Unlock()
}

// Generates a new "process ID" randomly.
// This identifier has only 32 bits of entropy, which means that collisions are not extremely unlikely.
// However, pids are only used as additional safeguards when acquiring locks on the reminders table, so in the (still rare) event of a collision, that is tolerable.
func generatePID() (string, error) {
	pidB := make([]byte, 4)
	_, err := io.ReadFull(rand.Reader, pidB)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(pidB), nil
}

type connectedHosts map[string]connectedHostInfo

type connectedHostInfo struct {
	serverMsgCh chan actorsv1pb.ServerStreamMessage
	actorTypes  []string
}

func (ch connectedHosts) updateCachedData(connectedHostsIDs, actorTypesRes []string) ([]string, []string) {
	if connectedHostsIDs == nil {
		// If the result slice is nil, allocate an initial capacity for the number of hosts
		connectedHostsIDs = make([]string, 0, len(ch))
	} else {
		// Reset the slice but keep the memory allocated
		connectedHostsIDs = connectedHostsIDs[:0]
	}

	if actorTypesRes == nil {
		// If the result slice is nil, allocate an initial capacity of 2 * number of hosts, as a guesstimate
		actorTypesRes = make([]string, 0, len(ch)*2)
	} else {
		// Reset the slice but keep the memory allocated
		actorTypesRes = actorTypesRes[:0]
	}

	foundTypes := make(map[string]struct{}, cap(actorTypesRes))
	for name, info := range ch {
		connectedHostsIDs = append(connectedHostsIDs, name)

		// Add the actor types avoiding duplicates
		for _, at := range info.actorTypes {
			_, ok := foundTypes[at]
			if ok {
				continue
			}
			foundTypes[at] = struct{}{}
			actorTypesRes = append(actorTypesRes, at)
		}
	}
	return connectedHostsIDs, actorTypesRes
}
