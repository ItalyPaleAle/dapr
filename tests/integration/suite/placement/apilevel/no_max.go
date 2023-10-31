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

package quorum

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	placementv1pb "github.com/dapr/dapr/pkg/proto/placement/v1"
	"github.com/dapr/dapr/tests/integration/framework"
	"github.com/dapr/dapr/tests/integration/framework/process/placement"
	"github.com/dapr/dapr/tests/integration/suite"
)

func init() {
	suite.Register(new(noMax))
}

// noMax tests placement reports API level with no maximum.
type noMax struct {
	place *placement.Placement
}

func (n *noMax) Setup(t *testing.T) []framework.Option {
	n.place = placement.New(t,
		placement.WithLogLevel("debug"),
	)

	return []framework.Option{
		framework.WithProcesses(n.place),
	}
}

func (n *noMax) Run(t *testing.T, ctx context.Context) {
	n.place.WaitUntilRunning(t, ctx)

	// Collect messages
	placementMessageCh := make(chan any)
	currentVersion := atomic.Uint32{}
	lastVersionUpdate := atomic.Int64{}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msgAny := <-placementMessageCh:
				switch msg := msgAny.(type) {
				case error:
					log.Printf("Received an error in the channel. This will make the test fail: '%v'", msg)
					return
				case uint32:
					old := currentVersion.Swap(msg)
					if old != msg {
						lastVersionUpdate.Store(time.Now().Unix())
					}
				}
			}
		}
	}()

	// Register the first host with API level 10
	ctx1, cancel1 := context.WithCancel(ctx)
	defer cancel1()
	registerHost(ctx1, n.place.Port(), 10, placementMessageCh)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, uint32(10), currentVersion.Load())
	}, 10*time.Second, 50*time.Millisecond)
	lastUpdate := lastVersionUpdate.Load()

	// Register the second host with API level 20
	ctx2, cancel2 := context.WithCancel(ctx)
	defer cancel2()
	registerHost(ctx2, n.place.Port(), 20, placementMessageCh)

	// After 3s, we should not receive an update
	// This can take a while as disseination happens on intervals
	time.Sleep(3 * time.Second)
	require.Equal(t, lastUpdate, lastVersionUpdate.Load())

	// Stop the first host, and the in API level should increase
	cancel1()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, uint32(20), currentVersion.Load())
	}, 10*time.Second, 50*time.Millisecond)

	// Trying to register a host with version 5 should fail
	registerHostFailing(t, ctx, n.place.Port(), 5)
}

// Expect the registration to fail with FailedPrecondition.
func registerHostFailing(t *testing.T, ctx context.Context, port int, apiLevel int) {
	// Establish a connection with placement
	conn, err := grpc.DialContext(ctx, "localhost:"+strconv.Itoa(port),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
	)
	require.NoError(t, err, "failed to establish gRPC connection")
	defer conn.Close()

	msg := &placementv1pb.Host{
		Name:     "myapp",
		Port:     1234,
		Entities: []string{"someactor"},
		Id:       "myapp",
		ApiLevel: uint32(apiLevel),
	}

	client := placementv1pb.NewPlacementClient(conn)
	stream, err := client.ReportDaprStatus(ctx)
	require.NoError(t, err, "failed to establish stream")

	err = stream.Send(msg)
	require.NoError(t, err, "failed to send message")

	// Should fail here
	_, err = stream.Recv()
	require.Error(t, err)
	require.Equalf(t, codes.FailedPrecondition, status.Code(err), "error was: %v", err)
}

func registerHost(ctx context.Context, port int, apiLevel int, placementMessage chan any) {
	// Establish a connection with placement
	conn, err := grpc.DialContext(ctx, "localhost:"+strconv.Itoa(port),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
	)
	if err != nil {
		placementMessage <- fmt.Errorf("failed to establish gRPC connection: %w", err)
		return
	}

	msg := &placementv1pb.Host{
		Name:     "myapp",
		Port:     1234,
		Entities: []string{"someactor"},
		Id:       "myapp",
		ApiLevel: uint32(apiLevel),
	}

	// Establish a stream and send the initial heartbeat
	// We need to retry here because this will fail until the instance of placement (the only one) acquires leadership
	var placementStream placementv1pb.Placement_ReportDaprStatusClient
	for j := 0; j < 4; j++ {
		client := placementv1pb.NewPlacementClient(conn)
		stream, rErr := client.ReportDaprStatus(ctx)
		if rErr != nil {
			log.Printf("Failed to connect to placement; will retry: %v", rErr)
			// Sleep before retrying
			time.Sleep(time.Second)
			continue
		}

		rErr = stream.Send(msg)
		if rErr != nil {
			log.Printf("Failed to send message; will retry: %v", rErr)
			_ = stream.CloseSend()
			// Sleep before retrying
			time.Sleep(time.Second)
			continue
		}

		// Receive the first message (which can't be an "update" one anyways) to ensure the connection is ready
		_, rErr = stream.Recv()
		if rErr != nil {
			log.Printf("Failed to receive message; will retry: %v", rErr)
			_ = stream.CloseSend()
			// Sleep before retrying
			time.Sleep(time.Second)
			continue
		}

		placementStream = stream
	}

	if placementStream == nil {
		placementMessage <- errors.New("did not connect to placement in time")
		return
	}

	// Send messages every second
	go func() {
		for {
			select {
			case <-ctx.Done():
				// Disconnect when the context is done
				placementStream.CloseSend()
				conn.Close()
				return
			case <-time.After(time.Second):
				placementStream.Send(msg)
			}
		}
	}()

	// Collect all API levels
	go func() {
		for {
			in, rerr := placementStream.Recv()
			if rerr != nil {
				if errors.Is(rerr, context.Canceled) || errors.Is(rerr, io.EOF) || status.Code(rerr) == codes.Canceled {
					// Stream ended
					placementMessage <- nil
					return
				}
				placementMessage <- fmt.Errorf("error from placement: %w", rerr)
			}
			if in.GetOperation() == "update" {
				placementMessage <- in.GetTables().GetApiLevel()
			}
		}
	}()
}
