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
	if daprPort == 0 {
		if s, _ := os.LookupEnv("DAPR_GRPC_PORT"); s != "" {
			daprPort, _ = strconv.Atoi(s)
		}
	}
	url := fmt.Sprintf("localhost:%d", daprPort)
	log.Printf("Connecting to dapr using url %s", url)

	var grpcConn *grpc.ClientConn
	start := time.Now()
	for retries := 10; retries > 0; retries-- {
		var err error
		grpcConn, err = grpc.Dial(url, grpc.WithInsecure(), grpc.WithBlock())
		if err == nil {
			break
		}

		if retries == 0 {
			log.Printf("Could not connect to dapr: %v", err)
			log.Panic(err)
		}

		log.Printf("Could not connect to dapr: %v, retrying...", err)
		time.Sleep(5 * time.Second)
	}

	elapsed := time.Since(start)
	log.Printf("gRPC connect elapsed: %v", elapsed)
	return runtimev1pb.NewDaprClient(grpcConn)
}
