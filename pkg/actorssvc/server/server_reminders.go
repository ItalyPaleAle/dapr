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
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dapr/components-contrib/actorstore"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
)

// CreateReminder creates a new reminder.
// If a reminder with the same ID (actor type, actor ID, name) already exists, it's replaced.
func (s *server) CreateReminder(ctx context.Context, req *actorsv1pb.CreateReminderRequest) (*actorsv1pb.CreateReminderResponse, error) {
	if !s.opts.EnableReminders {
		return nil, status.Error(codes.PermissionDenied, "Reminders functionality is not enabled")
	}

	err := req.GetReminder().ValidateRequest()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid reminder in request: %v", err)
	}

	err = s.store.CreateReminder(ctx, req.ToActorStoreRequest())
	if err != nil {
		log.Errorf("Failed to create reminder: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create reminder: %v", err)
	}

	return &actorsv1pb.CreateReminderResponse{}, nil
}

// GetReminder returns details about an existing reminder.
func (s *server) GetReminder(ctx context.Context, req *actorsv1pb.GetReminderRequest) (*actorsv1pb.GetReminderResponse, error) {
	if !s.opts.EnableReminders {
		return nil, status.Error(codes.PermissionDenied, "Reminders functionality is not enabled")
	}

	err := req.GetRef().Validate()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid reminder reference in request: %v", err)
	}

	res, err := s.store.GetReminder(ctx, req.Ref.ToActorStoreReminderRef())
	if err != nil {
		if errors.Is(err, actorstore.ErrReminderNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		log.Errorf("Failed to get reminder: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to get reminder: %v", err)
	}

	reminder := &actorsv1pb.Reminder{
		ActorType:     req.Ref.ActorType,
		ActorId:       req.Ref.ActorId,
		Name:          req.Ref.Name,
		ExecutionTime: timestamppb.New(res.ExecutionTime),
	}
	if res.Period != nil {
		reminder.Period = *res.Period
	}
	if res.TTL != nil {
		reminder.Ttl = timestamppb.New(*res.TTL)
	}
	if len(res.Data) > 0 {
		reminder.Data = res.Data
	}
	return &actorsv1pb.GetReminderResponse{
		Reminder: reminder,
	}, nil
}

// DeleteReminder removes an existing reminder before it fires.
func (s *server) DeleteReminder(ctx context.Context, req *actorsv1pb.DeleteReminderRequest) (*actorsv1pb.DeleteReminderResponse, error) {
	if !s.opts.EnableReminders {
		return nil, status.Error(codes.PermissionDenied, "Reminders functionality is not enabled")
	}

	err := req.GetRef().Validate()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid reminder reference in request: %v", err)
	}

	err = s.store.DeleteReminder(ctx, req.Ref.ToActorStoreReminderRef())
	if err != nil {
		if errors.Is(err, actorstore.ErrReminderNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		log.Errorf("Failed to delete reminder: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to delete reminder: %v", err)
	}

	return &actorsv1pb.DeleteReminderResponse{}, nil
}

// ReminderCompleted is used by the sidecar to acknowledge that a reminder has been executed successfully.
func (s *server) ReminderCompleted(ctx context.Context, req *actorsv1pb.ReminderCompletedRequest) (*actorsv1pb.ReminderCompletedResponse, error) {
	if !s.opts.EnableReminders {
		return nil, status.Error(codes.PermissionDenied, "Reminders functionality is not enabled")
	}

	panic("unimplemented")
}
