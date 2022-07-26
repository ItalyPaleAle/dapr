package utils

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"

	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
)

// GetGRPCClient returns a gRPC client to connect to Dapr
func GetGRPCClient(daprPort int) runtimev1pb.DaprClient {
	if daprPort <= 0 {
		if s, _ := os.LookupEnv("DAPR_GRPC_PORT"); s != "" {
			daprPort, _ = strconv.Atoi(s)
		}
	}
	if daprPort <= 0 {
		log.Panic("Empty value for Dapr gRPC port")
	}

	grpcConn, err := GetGRPCClientConnection(daprPort)
	if err != nil {
		log.Panic(err)
	}

	return GetDaprClientForConnection(grpcConn)
}

func GetGRPCClientConnection(daprPort int) (grpcConn *grpc.ClientConn, err error) {
	url := fmt.Sprintf("localhost:%d", daprPort)
	log.Printf("Connecting to Dapr gRPC using url %s", url)

	start := time.Now()
	for retries := 10; retries > 0; retries-- {
		grpcConn, err = grpc.Dial(url, grpc.WithInsecure(), grpc.WithBlock())
		if err == nil {
			break
		}

		if retries == 0 {
			return nil, fmt.Errorf("Could not connect to Dapr: %w", err)
		}

		log.Printf("Could not connect to Dapr: %v, retrying…", err)
		time.Sleep(5 * time.Second)
	}

	elapsed := time.Since(start)
	log.Printf("gRPC connect elapsed: %v", elapsed)

	return grpcConn, nil
}

func GetDaprClientForConnection(grpcConn *grpc.ClientConn) runtimev1pb.DaprClient {
	return runtimev1pb.NewDaprClient(grpcConn)
}
