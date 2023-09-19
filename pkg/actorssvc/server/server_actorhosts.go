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

	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
)

// ConnectHost is used by the Dapr sidecar to register itself as an actor host.
// It remains active as a long-lived bi-di stream to allow for the Actors service
// to communicate with the sidecar.
func (s *server) ConnectHost(ss actorsv1pb.Actors_ConnectHostServer) error {
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
