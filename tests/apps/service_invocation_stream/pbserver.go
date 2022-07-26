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
	"fmt"
	"io"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "service_invocation_stream/pb"
)

func StartTesterServer(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s := grpc.NewServer()

	pb.RegisterTesterServer(s, &testerServer{})
	log.Printf("testerServer listening on %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

func GetTesterClient(conn *grpc.ClientConn) pb.TesterClient {
	return pb.NewTesterClient(conn)
}

// testerServer is used to implement pb.TesterServer.
type testerServer struct {
	pb.UnimplementedTesterServer
}

func (s *testerServer) TestDirector(ss pb.Tester_TestDirectorServer) error {
	done := make(chan error)
	go func() {
		// Receiver must wait for messages from the director app
		for ss.Context().Err() == nil {
			// This call is blocking
			in, err := ss.Recv()
			if err == io.EOF {
				log.Println("Stream reached EOF")
				done <- nil
				break
			} else if err != nil {
				log.Println("Error while reading message:", err)
				done <- err
				break
			}
		}
	}()

	// Wait for connection to be closed
	select {
	case <-ss.Context().Done():
		// Connection closed
		return nil
	case err := <-done:
		return err
	}
}
