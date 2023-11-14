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
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
)

// CreateReminder creates a new reminder.
// If a reminder with the same ID (actor type, actor ID, name) already exists, it's replaced.
func (s *server) CreateReminder(ctx context.Context, req *actorsv1pb.CreateReminderRequest) (*actorsv1pb.CreateReminderResponse, error) {
	if !s.opts.EnableReminders {
		return nil, status.Error(codes.PermissionDenied, "Reminders functionality is not enabled")
	}

	reminder := req.GetReminder()
	if err := reminder.ValidateRequest(); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid reminder in request: %v", err)
	}

	delay := reminder.GetExecutionTimeDelay(s.clock.Now())

	log.Debugf("Invoked CreateReminder: key='%s' executionTime='%v' delay='%v' period='%s' ttl='%v' data='%d bytes'", reminder.GetKey(), reminder.ExecutionTime, delay, reminder.Period, reminder.Ttl, len(reminder.Data))

	// If the reminder is scheduled to be executed in the fetchAhead interval, and it can be delivered to an actor host connected to this instance, acquire a lease too
	if delay > s.opts.RemindersFetchAheadInterval {
		// This reminder's execution time is beyond the fetchAhead interval, so we just add it
		err := s.store.CreateReminder(ctx, req.ToActorStoreRequest())
		if err != nil {
			log.Errorf("Failed to create reminder %s: %v", reminder.GetKey(), err)
			return nil, status.Errorf(codes.Internal, "failed to create reminder: %v", err)
		}

		// Remove from the queue in case it's a reminder that's been updated and it was in there
		// We ignore errors here
		_ = s.processor.Dequeue(reminder.GetKey())

		return &actorsv1pb.CreateReminderResponse{}, nil
	}

	storeReq := actorstore.CreateLeasedReminderRequest{
		Reminder: req.ToActorStoreRequest(),
	}
	s.connectedHostsLock.RLock()
	// Note we are limiting ourselves to non-paused hosts here
	storeReq.Hosts = s.connectedHostsIDs
	storeReq.ActorTypes = s.connectedHostsActorTypes
	s.connectedHostsLock.RUnlock()

	fetched, err := s.store.CreateLeasedReminder(ctx, storeReq)
	if err != nil {
		log.Errorf("Failed to create reminder %s: %v", reminder.GetKey(), err)
		return nil, status.Errorf(codes.Internal, "failed to create reminder: %v", err)
	}

	// Enqueue the reminder
	if fetched != nil {
		err = s.enqueueReminder(fetched)
		if err != nil {
			log.Errorf("Failed to enqueue reminder %s: %v", reminder.GetKey(), err)
			return nil, status.Errorf(codes.Internal, "failed to enqueue reminder: %v", err)
		}
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

	log.Debugf("Invoked GetReminder with key='%s'", req.GetRef().GetKey())

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
	if res.Period != nil && *res.Period != "" {
		// Internally, for reminders that have a finite amount of repetitions, we add a counter at the end
		// We need to remove that
		reminder.Period, _, _ = strings.Cut(*res.Period, "||")
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

	log.Debugf("Invoked DeleteReminder with key='%s'", req.GetRef().GetKey())

	// Remove from the queue if present
	// We ignore errors here
	_ = s.processor.Dequeue(req.Ref.GetKey())

	err = s.store.DeleteReminder(ctx, req.Ref.ToActorStoreReminderRef())
	if err != nil {
		if errors.Is(err, actorstore.ErrReminderNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		log.Errorf("Failed to delete reminder: %v", err)
		return nil, status.Errorf(codes.Internal, "Failed to delete reminder: %v", err)
	}

	return &actorsv1pb.DeleteReminderResponse{}, nil
}

// CompleteReminder is sent to report that a reminder has been completed.
// Hosts are expected to inform the Actors service when a reminder is completed, so it's possible to guarantee at-least-once delivery.
// Repeating reminders are re-queued for the next execution, and non-repeating reminders are deleted.
func (s *server) CompleteReminder(ctx context.Context, req *actorsv1pb.CompleteReminderRequest) (*actorsv1pb.CompleteReminderResponse, error) {
	if !s.opts.EnableReminders {
		return nil, status.Error(codes.PermissionDenied, "Reminders functionality is not enabled")
	}

	err := req.GetRef().Validate()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid reminder reference in request: %v", err)
	}
	if req.CompletionToken == "" {
		return nil, status.Error(codes.InvalidArgument, "Completion token is empty in request")
	}

	log.Debugf("Invoked CompleteReminder with key='%s'", req.GetRef().GetKey())

	// Get the active reminder from the cache and delete it at the same time
	// If needed, we can always add it back, and the "happy path" is more efficient
	// (if this instance were to crash, that would make the lock invalid anyways, so there's no risk there; additionally, this makes duplicate requests fail right away too)
	arMapKey := req.CompletionToken + "||" + req.Ref.GetKey()
	ar, ok := s.activeReminders.GetAndDel(arMapKey)
	if !ok || ar == nil {
		log.Warnf("Reminder and/or completion token not found: '%s'", ar.fr.Key())
		return nil, status.Error(codes.NotFound, "Reminder not found")
	}

	// If the reminder doesn't repeat, or if the request is for the reminder to be stopped, delete the reminder
	// Otherwise, we'll update its execution time
	nextExecutionTime, updatedPeriod, doesRepeat := ar.reminderNextExecutionTime(ar.start)
	if req.StopReminder || !doesRepeat {
		// Reminder doesn't repeat, so we just delete it
		err = s.store.DeleteReminderWithLease(ctx, ar.fr)

		if err != nil {
			if errors.Is(err, actorstore.ErrReminderNotFound) {
				// If the reminder can't be found, it means that it's been deleted or updated in parallel
				// In this case, we can ignore the error
				return &actorsv1pb.CompleteReminderResponse{}, nil
			}

			// Re-add the object to the map
			s.activeReminders.Set(arMapKey, ar)

			log.Errorf("Failed to delete reminder '%s': %v", ar.fr.Key(), err)
			return nil, status.Errorf(codes.Internal, "Failed to delete reminder: %v", err)
		}

		return &actorsv1pb.CompleteReminderResponse{}, nil
	}

	// If we're here, the reminder repeats so we need to update it
	updateReq := actorstore.UpdateReminderWithLeaseRequest{
		ExecutionTime: nextExecutionTime,
		TTL:           ar.reminder.TTL,
	}
	if updatedPeriod != "" {
		updateReq.Period = &updatedPeriod
	}

	// If the reminder is scheduled to be repeated within the fetch ahead interval, we do not relinquish the lease
	// This will also make us re-enqueue the reminder immediately
	if nextExecutionTime.Sub(s.clock.Now()) <= s.opts.RemindersFetchAheadInterval {
		updateReq.KeepLease = true
	}

	// Perform the update
	err = s.store.UpdateReminderWithLease(ctx, ar.fr, updateReq)
	if err != nil {
		if errors.Is(err, actorstore.ErrReminderNotFound) {
			// If the reminder can't be found, it means that it's been deleted or updated in parallel
			// In this case, we can ignore the error
			return &actorsv1pb.CompleteReminderResponse{}, nil
		}

		// Re-add the object to the map
		s.activeReminders.Set(arMapKey, ar)

		log.Errorf("Failed to update reminder '%s': %v", ar.fr.Key(), err)
		return nil, status.Errorf(codes.Internal, "Failed to update reminder: %v", err)
	}

	// If we still have the lease, re-enqueue the reminder
	if updateReq.KeepLease {
		newFr := actorstore.NewFetchedReminder(ar.fr.Key(), nextExecutionTime, ar.fr.Lease())
		// This can't really fail
		_ = s.processor.Enqueue(&newFr)
	}

	log.Debugf("Reminder '%s' was executed in %dms", req.GetRef().GetKey(), time.Since(ar.start).Milliseconds())

	return &actorsv1pb.CompleteReminderResponse{}, nil
}
