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

// ReminderCompleted is used by the sidecar to acknowledge that a reminder has been executed successfully.
func (s *server) ReminderCompleted(context.Context, *actorsv1pb.ReminderCompletedRequest) (*actorsv1pb.ReminderCompletedResponse, error) {
	panic("unimplemented")
}
