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
	"sync/atomic"

	"github.com/alphadose/haxmap"
	"google.golang.org/grpc"
	kclock "k8s.io/utils/clock"

	"github.com/dapr/dapr/pkg/actors/cache"
	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/kit/events/queue"
	"github.com/dapr/kit/logger"
)

var log = logger.NewLogger("dapr.actorssvc.server")

// server is the gRPC server for the Actors service.
type server struct {
	opts       Options
	store      actorstore.Store
	srv        *grpc.Server
	shutdownCh chan struct{}
	clock      kclock.WithTicker
	processor  *queue.Processor[*actorstore.FetchedReminder]
	cache      *cache.Cache[*actorsv1pb.LookupActorResponse]

	// "Process ID", which is generated randomly when the server is initialized.
	pid string

	// This map contains the list of active connections from actor hosts.
	// We use a "regular" map with a RWMutex instead of a sync.Map because we need to be sure that once we get a channel from the map, it's still valid when we attempt to use it.
	connectedHosts     connectedHosts
	connectedHostsLock sync.RWMutex

	// Map that contains the currently-active reminders
	// The key is a string in the format "hostId||reminderType||reminderId||reminderName||completionKey"
	activeReminders *haxmap.Map[string, *activeReminder]

	// Current API level for actors in the cluster
	clusterAPILevel atomic.Uint32

	// Cached values of host IDs and actor types for connected hosts.
	// We maintain a cache here to improve memory usage when we need to fetch reminders etc, and to reduce the duration we need to acquire read locks for.
	// Note1: When grabbing these slices, make a copy of the pointer while obtaining a (read) lock on connectedHostsLock, for example `hostIDs := s.connectedHostsIDs`. Do not modify the value of these slices in any way.
	// Note2: These should be used for *reminders* only, as these values exclude hosts that are in a "paused" state due to too many reminders being delivered. If you need to get the list of connected hosts for other purposes, you should iterate through connectedHosts directly (after obtaining the connectedHostsLock!).
	connectedHostsIDs        []string
	connectedHostsActorTypes []string
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
	s.activeReminders = haxmap.New[string, *activeReminder]()

	s.clock = opts.clock
	if s.clock == nil {
		s.clock = kclock.RealClock{}
	}

	// Generate a random PID
	s.pid, err = generatePID()
	if err != nil {
		return fmt.Errorf("failed to generate random process ID: %w", err)
	}

	log.Infof("Actors subsystem configuration: %v", s.opts.GetActorsConfiguration())

	// Init the store
	err = s.initActorStore(ctx)
	if err != nil {
		return fmt.Errorf("failed to init actor store: %w", err)
	}

	// If reminder processing is enabled, start polling for reminder in background
	if s.opts.EnableReminders {
		go s.startReminders(ctx)
	}

	// Init the cache for actors
	// This has a max TTL of 20s
	s.cache = cache.NewCache[*actorsv1pb.LookupActorResponse](cache.CacheOptions{
		MaxTTL: 20,
	})

	// Create the gRPC server
	s.srv = grpc.NewServer(opts.Security.GRPCServerOptionMTLS())
	actorsv1pb.RegisterActorsServer(s.srv, s)

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

	s.store.SetOnActorsAPILevelUpdate(func(apiLevel uint32) {
		if s.clusterAPILevel.Swap(apiLevel) == apiLevel {
			// Value was unchanged
			return
		}

		// TODO: Notify all connected hosts
	})

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
