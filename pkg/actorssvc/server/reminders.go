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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dapr/components-contrib/actorstore"
	"github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/kit/events/queue"
	"github.com/dapr/kit/logger"
	timeutils "github.com/dapr/kit/time"
)

func (s *server) pollForReminders(ctx context.Context) {
	log.Infof("Start polling for reminder with interval='%v' fetchAheadInterval='%v' batchSize='%d' leaseDuration='%v'", s.opts.RemindersPollInterval, s.opts.RemindersFetchAheadInterval, s.opts.RemindersFetchAheadBatchSize, s.opts.RemindersLeaseDuration)

	s.processor = queue.NewProcessor[*actorstore.FetchedReminder](s.executeReminder)
	defer func() {
		err := s.processor.Close()
		if err != nil {
			log.Errorf("Failed to stop queue processor: %v", err)
		}
	}()

	ticker := s.clock.NewTicker(s.opts.RemindersPollInterval)
	defer ticker.Stop()

	var failureCount int
	for {
		select {
		case <-ticker.C():
			// Fetch the reminders
			err := s.scheduleNewReminders(ctx)
			if err != nil {
				failureCount++
				log.Errorf("Failed to schedule reminders: %v", err)

				// After 5 consecutive failures, crash
				if failureCount == 5 {
					log.Fatalf("Failed fetching reminders %d times in sequence", failureCount)
				}
			} else {
				// Reset failureCount
				failureCount = 0
			}

		case <-ctx.Done():
			log.Info("Stopped polling for reminder")
			return
		}
	}
}

func (s *server) scheduleNewReminders(ctx context.Context) error {
	reminders, err := s.fetchReminders(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch reminders: %w", err)
	}

	for i := range reminders {
		err = s.enqueueReminder(reminders[i])
		if err != nil {
			return fmt.Errorf("failed to enqueue reminder %s: %w", reminders[i].Key(), err)
		}
	}

	return nil
}

func (s *server) enqueueReminder(r *actorstore.FetchedReminder) error {
	if log.IsOutputLevelEnabled(logger.DebugLevel) {
		log.Debugf("Scheduling reminder '%s' to be executed at '%v'", r.Key(), r.ScheduledTime())
	}
	err := s.processor.Enqueue(r)
	if err != nil {
		return err
	}
	return nil
}

func (s *server) executeReminder(fr *actorstore.FetchedReminder) {
	// Executing reminders is a multi-step process
	// First, we fetch the reminder again, to be sure that the data we have in-memory is up-to-date:
	// it is possible, for example, that another instance of the actors service may have modified the reminder in the database
	// before we got to this stage.
	// Note that after this check completes, the reminder *will* be executed (or attempted to be executed). If someone else modifies the reminder after this check (but before step 3), the original reminder will still be executed (and modifications would only impact subsequent iterations).
	// Second, we send the reminder to the actor host that is currently hosting the actor (or if the actor is inactive, we activate it in one of the hosts that we are connected to).
	// Third, and last, we remove the reminder from the reminders table. However, if the reminder is repeating (and hasn't reached its TTL), then we update its execution time instead.

	ctx := context.Background()
	log.Debugf("Executing reminder '%s'…", fr.Key())
	start := time.Now()

	// Start by retrieving the reminder's data
	reminder, err := s.store.GetReminderWithLease(ctx, fr)
	if err != nil {
		if errors.Is(err, actorstore.ErrReminderNotFound) {
			// If the reminder can't be found now, it means that either the lease was lost, or more likely the reminder was modified in the database
			// In either case, we can ignore that error.
			log.Debugf("Reminder '%s' was modified in the store and will not be executed", fr.Key())
			return
		}

		log.Errorf("Failed to retrieve reminder '%s' with lease: %v", fr.Key(), err)
		return
	}

	// Lookup the host ID for the actor
	s.connectedHostsLock.RLock()
	connectedHosts := s.connectedHostsIDs
	s.connectedHostsLock.RUnlock()
	lar, err := s.store.LookupActor(ctx, reminder.ActorRef(), actorstore.LookupActorOpts{
		Hosts: connectedHosts,
	})
	if err != nil {
		if errors.Is(err, actorstore.ErrNoActorHost) {
			// If there's no host capable of serving this reminder, it means that the host has disconnected since we fetched the reminder
			// In this case, we can ignore the error
			log.Debugf("Reminder '%s' cannot be executed on any host currently connected to this instance and will be retried later", fr.Key())
			return
		}

		log.Errorf("Failed to lookup actor host for reminder '%s': %v", fr.Key(), err)
		return
	}

	// Send the message to the actor host
	serverMsg := &actors.ConnectHostServerStream_ExecuteReminder{
		ExecuteReminder: &actors.ExecuteReminder{
			Reminder: actors.NewReminderFromActorStore(reminder),
		},
	}
	s.connectedHostsLock.RLock()
	connHostInfo, ok := s.connectedHosts[lar.HostID]
	s.connectedHostsLock.RUnlock()
	if !ok {
		// Same situation as above: the host has disconnected since we fetched the reminder
		log.Debugf("Reminder '%s' cannot be executed on any host currently connected to this instance and will be retried later", fr.Key())
		return
	}

	// Sanity check to ensure the channel isn't blocked for too long, leading to a goroutine leak
	sendCtx, sendCancel := context.WithTimeout(ctx, 5*time.Second)
	defer sendCancel()
	select {
	case connHostInfo.serverMsgCh <- serverMsg:
		// All good - nop
	case <-sendCtx.Done():
		log.Errorf("Failed to send reminder '%s' to actor host: %v", fr.Key(), ctx.Err())
		return
	}

	// Lastly, remove the reminder from the store (or update the execution time for repeating reminders)
	nextExecutionTime, updatedPeriod, doesRepeat := s.reminderNextExecutionTime(reminder)
	if doesRepeat {
		updateReq := actorstore.UpdateReminderWithLeaseRequest{
			ExecutionTime: nextExecutionTime,
			TTL:           reminder.TTL,
		}
		if updatedPeriod != "" {
			updateReq.Period = &updatedPeriod
		}

		// If the reminder is scheduled to be repeated within the fetch ahead interval, we do not release the lease
		if nextExecutionTime.Sub(s.clock.Now()) <= s.opts.RemindersFetchAheadInterval {
			updateReq.KeepLease = true
		}

		err = s.store.UpdateReminderWithLease(ctx, fr, updateReq)

		// If the reminder can't be found, it means that it's been deleted or updated in parallel
		// In this case, we can ignore the error
		if err != nil && !errors.Is(err, actorstore.ErrReminderNotFound) {
			log.Errorf("Failed to update reminder '%s': %v", fr.Key(), err)
			return
		}

		// If we still have the lease, re-enqueue the reminder
		if updateReq.KeepLease {
			newFr := actorstore.NewFetchedReminder(fr.Key(), nextExecutionTime, fr.Lease())
			err = s.processor.Enqueue(&newFr)
			if err != nil {
				// Log the warning only
				log.Warnf("Failed to re-enqueue repeating reminder '%s': %v", fr.Key(), err)
			}
		}
	} else {
		err = s.store.DeleteReminderWithLease(ctx, fr)

		// If the reminder can't be found, it means that it's been deleted or updated in parallel
		// In this case, we can ignore the error
		if err != nil && !errors.Is(err, actorstore.ErrReminderNotFound) {
			log.Errorf("Failed to delete reminder '%s': %v", fr.Key(), err)
			return
		}
	}

	log.Debugf("Reminder '%s' has been executed in %v", fr.Key(), time.Since(start))
}

func (s *server) fetchReminders(ctx context.Context) ([]*actorstore.FetchedReminder, error) {
	s.connectedHostsLock.RLock()
	req := actorstore.FetchNextRemindersRequest{
		Hosts:      s.connectedHostsIDs,
		ActorTypes: s.connectedHostsActorTypes,
	}
	s.connectedHostsLock.RUnlock()

	if len(req.Hosts) == 0 && len(req.ActorTypes) == 0 {
		// Short-circuit if there's nothing to load
		return nil, nil
	}

	return s.store.FetchNextReminders(ctx, req)
}

func (s *server) reminderNextExecutionTime(reminder actorstore.Reminder) (next time.Time, updatedPeriod string, doesRepeat bool) {
	// Reminder does not repeat
	if reminder.Period == nil || *reminder.Period == "" {
		return next, "", false
	}

	// For reminders that have a finite number of repetitions, we append "||<num>" at the end to count the ones that have been executed
	// We need to remove that before returning
	var (
		executedCount int
		countStr      string
	)
	updatedPeriod = *reminder.Period
	updatedPeriod, countStr, _ = strings.Cut(updatedPeriod, "||")
	if countStr != "" {
		executedCount, _ = strconv.Atoi(countStr)
	}
	executedCount++

	years, months, days, period, repeats, err := timeutils.ParseDuration(*reminder.Period)
	if err != nil || repeats == 0 {
		// The repetition string should have been parsed when the reminder was created, to guarantee it's valid
		// So this should never happen
		return next, "", false
	}

	if repeats > 0 && executedCount >= repeats {
		// We have exhausted all repetitions
		return next, "", false
	}

	// Calculate the next repetition time
	// Note this starts from the current time and not the previous execution time
	next = s.clock.Now().AddDate(years, months, days).Add(period)

	// If the next repetition is after the TTL, we stop repeating
	if reminder.TTL != nil && !reminder.TTL.IsZero() && next.After(*reminder.TTL) {
		return next, "", false
	}

	// Append the execution count if we are tracking repetitions
	if updatedPeriod != "" && countStr != "" && executedCount > 0 {
		updatedPeriod += "||" + strconv.Itoa(executedCount)
	}

	return next, updatedPeriod, true
}
