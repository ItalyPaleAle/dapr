/*
Copyright 2021 The Dapr Authors
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

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/metadata"

	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/dapr/dapr/tests/apps/utils"

	"service_invocation_stream/pb"
)

const (
	calleeAppA = "stream-callee-a"
	calleeAppB = "stream-callee-b"
)

var (
	daprHTTPPort = 3500
	daprGRPCPort = 50001
	appHTTPPort  = 3000
	appGRPCPort  = 3000

	httpClient     = utils.NewHTTPClient()
	daprGrpcClient runtimev1pb.DaprClient
	testerClient   pb.TesterClient
)

func init() {
	var p string
	p = os.Getenv("DAPR_HTTP_PORT")
	if p != "" && p != "0" {
		daprHTTPPort, _ = strconv.Atoi(p)
	}
	p = os.Getenv("DAPR_GRPC_PORT")
	if p != "" && p != "0" {
		daprGRPCPort, _ = strconv.Atoi(p)
	}
	p = os.Getenv("APP_HTTP_PORT")
	if p != "" && p != "0" {
		appHTTPPort, _ = strconv.Atoi(p)
	}
	p = os.Getenv("APP_GRPC_PORT")
	if p != "" && p != "0" {
		appGRPCPort, _ = strconv.Atoi(p)
	}
}

type appResponse struct {
	Message string `json:"message,omitempty"`
}

func runInvokeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("runInvokeHandler is called\n")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Create a new context for this test
	testID := uuid.NewString()
	ctx = metadata.AppendToOutgoingContext(ctx, "test-id", testID)

	w.WriteHeader(http.StatusOK)
}

func messageStreamHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("messageStreamHandler is called\n")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(appResponse{Message: "OK"})
}

// indexHandler is the handler for root path
func indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("indexHandler is called\n")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(appResponse{Message: "OK"})
}

// appRouter initializes restful api router
func appRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	// Log requests and their processing time
	router.Use(utils.LoggerMiddleware)

	router.HandleFunc("/", indexHandler).Methods("GET")

	// These are called by the test runner and trigger tests
	router.HandleFunc("/run/invoke", runInvokeHandler).Methods("POST")

	// These are called through Dapr service invocation
	router.HandleFunc("/message/stream", messageStreamHandler).Methods("POST")

	return router
}

func main() {
	grpcConn, err := utils.GetGRPCClientConnection(daprGRPCPort)
	if err != nil {
		panic(err)
	}
	daprGrpcClient = utils.GetDaprClientForConnection(grpcConn)
	testerClient = GetTesterClient(grpcConn)
	_ = daprGrpcClient

	// Start the gRPC server if needed
	if appGRPCPort > 0 {
		go func() {
			err := StartTesterServer(appGRPCPort)
			if err != nil {
				log.Fatalf("Failed to start gRPC server: %v", err)
			}
		}()
	}

	log.Printf("HTTP server listening on http://localhost:%d", appHTTPPort)
	utils.StartServer(appHTTPPort, appRouter, true)
}
