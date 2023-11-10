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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dapr/components-contrib/actorstore"
	"github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/kit/events/queue"
	"github.com/dapr/kit/logger"
	timeutils "github.com/dapr/kit/time"
)

// Type for the data stored in activeRemindersMap
type activeReminder struct {
	fr       *actorstore.FetchedReminder
	reminder actorstore.Reminder
	start    time.Time
}

func (s *server) startReminders(ctx context.Context) {
	log.Infof("Start polling for reminder with interval='%v' fetchAheadInterval='%v' batchSize='%d' leaseDuration='%v'", s.opts.RemindersPollInterval, s.opts.RemindersFetchAheadInterval, s.opts.RemindersFetchAheadBatchSize, s.opts.RemindersLeaseDuration)

	s.processor = queue.NewProcessor[*actorstore.FetchedReminder](s.executeReminder)
	defer func() {
		err := s.processor.Close()
		if err != nil {
			log.Errorf("Failed to stop queue processor: %v", err)
		}
	}()

	// Renew leases for reminders in background
	go s.renewReminderLeases(ctx)

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

func (s *server) enqueueReminder(fr *actorstore.FetchedReminder) error {
	// If the reminder is to be executed right away, skip the processor
	if time.Until(fr.ScheduledTime()) < 500*time.Microsecond {
		go s.executeReminder(fr)
		return nil
	}

	if log.IsOutputLevelEnabled(logger.DebugLevel) {
		log.Debugf("Scheduling reminder '%s' to be executed at '%v'", fr.Key(), fr.ScheduledTime())
	}

	// This can't really fail
	_ = s.processor.Enqueue(fr)
	return nil
}

func (s *server) executeReminder(fr *actorstore.FetchedReminder) {
	// Executing reminders is a multi-step process
	// First, we fetch the reminder again, to be sure that the data we have in-memory is up-to-date:
	// it is possible, for example, that another instance of the actors service may have modified the reminder in the database
	// before we got to this stage.
	// Note that after this check completes, the reminder *will* be executed (or attempted to be executed). If someone else modifies the reminder after this check (but before step 3), the original reminder will still be executed (and modifications would only impact subsequent iterations).
	// Second, we add the reminder to the active reminders table.
	// Third, and last, we send the reminder to the actor host that is currently hosting the actor (or if the actor is inactive, we activate it in one of the hosts that we are connected to).

	ctx := context.Background()
	log.Debugf("Executing reminder '%s'", fr.Key())
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
	// Note that we are limiting ourselves to non-paused hosts
	connectedHosts := s.connectedHostsIDs
	s.connectedHostsLock.RUnlock()
	lar, err := s.store.LookupActor(ctx, reminder.ActorRef(), actorstore.LookupActorOpts{
		Hosts: connectedHosts,
	})
	if err != nil {
		if errors.Is(err, actorstore.ErrNoActorHost) {
			// If there's no host capable of serving this reminder, it means that the host has disconnected since we fetched the reminder
			// In this case, we can ignore the error
			s.executeReminderRelinquishLease(ctx, fr)
			return
		}

		log.Errorf("Failed to lookup actor host for reminder '%s': %v", fr.Key(), err)
		return
	}

	// Get the actors.Reminder object
	ar := actors.NewReminderFromActorStore(reminder)

	// Generate a completion token, as a simple 64-bit random value with the prefix of the host ID
	// Collision-resistance isn't important here as there can only be a single reminder with the given actor and name
	completionTokenBytes := make([]byte, 8)
	_, err = io.ReadFull(rand.Reader, completionTokenBytes)
	if err != nil {
		log.Errorf("Failed to generate random completion token for reminder '%s': %v", fr.Key(), err)
		return
	}
	completionToken := lar.HostID + "||" + base64.RawURLEncoding.EncodeToString(completionTokenBytes)

	// Send the message to the actor host
	serverMsg := &actors.ConnectHostServerStream_ExecuteReminder{
		ExecuteReminder: &actors.ExecuteReminder{
			Reminder:        ar,
			CompletionToken: completionToken,
		},
	}
	s.connectedHostsLock.RLock()
	connHostInfo, ok := s.connectedHosts[lar.HostID]
	s.connectedHostsLock.RUnlock()
	if !ok {
		// Same situation as above: the host has disconnected since we fetched the reminder
		s.executeReminderRelinquishLease(ctx, fr)
		return
	}

	// Store the active reminder
	arMapKey := completionToken + "||" + ar.GetKey()
	s.activeReminders.Set(arMapKey, &activeReminder{
		fr:       fr,
		reminder: reminder,
		start:    start,
	})

	// Sanity check to ensure the channel isn't blocked for too long, leading to a goroutine leak
	sendCtx, sendCancel := context.WithTimeout(ctx, 5*time.Second)
	defer sendCancel()
	select {
	case connHostInfo.serverMsgCh <- serverMsg:
		// All good - nop
	case <-sendCtx.Done():
		log.Errorf("Failed to send reminder '%s' to actor host: %v", fr.Key(), ctx.Err())

		// Remove from the map
		// Note it can take a few moments for this to appear if the map was being resized when the item was set
		// We still set a limit on the number of attempts in case the reminder is being deleted in parallel somewhere else
		for i := 0; i < 10; i++ {
			_, ok = s.activeReminders.GetAndDel(arMapKey)
			if ok {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		return
	}

	log.Debugf("Reminder '%s' is being executed on host '%s'", fr.Key(), lar.HostID)
}

func (s *server) executeReminderRelinquishLease(ctx context.Context, fr *actorstore.FetchedReminder) {
	log.Debugf("Reminder '%s' cannot be executed on any host currently connected to this instance and will be retried later", fr.Key())

	err := s.store.RelinquishReminderLease(ctx, fr)

	// Here, we log errors only, since this method is run for cleanup reasons already
	// If the error is that the reminder can't be found, it means that it's been deleted or updated in parallel
	// In this case, we can ignore the error
	if err != nil && !errors.Is(err, actorstore.ErrReminderNotFound) {
		log.Errorf("Failed to relinquish lease for reminder %s: %w", fr.Key(), err)
	}
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

func (s *server) renewReminderLeases(ctx context.Context) {
	// Renew leases every half of the lease duration, or up to 10s less than the lease duration
	var renewInterval time.Duration
	if s.opts.RemindersLeaseDuration >= 20*time.Second {
		renewInterval = s.opts.RemindersLeaseDuration - 10*time.Second
	} else {
		renewInterval = s.opts.RemindersLeaseDuration / 2
	}

	log.Infof("Renewing leases for reminders every %v", renewInterval)

	ticker := s.clock.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C():
			// Renew the leases
			s.connectedHostsLock.RLock()
			req := actorstore.RenewReminderLeasesRequest{
				Hosts:      s.connectedHostsIDs,
				ActorTypes: s.connectedHostsActorTypes,
			}
			s.connectedHostsLock.RUnlock()
			count, err := s.store.RenewReminderLeases(ctx, req)
			if err != nil {
				log.Errorf("failed to renew leases for reminders: %v", err)
			}

			// We do not check if the number of renewed reminders is the same as the count of reminders we have in the queue, because that's a recipe for failure due to race conditions with reminders being executed at the same time, and reminders being modified by other instances
			// Let's just use the data for a nice debug log instead
			if count > 0 {
				log.Debugf("Renewed leases for %d reminders", count)
			}

		case <-ctx.Done():
			// Stop when the context is done
			return
		}
	}
}

func (ar activeReminder) reminderNextExecutionTime(start time.Time) (next time.Time, updatedPeriod string, doesRepeat bool) {
	reminder := ar.reminder

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
	// Note this starts from the previous execution time
	next = start.AddDate(years, months, days).Add(period)

	// If the next repetition is after the TTL, we stop repeating
	if reminder.TTL != nil && !reminder.TTL.IsZero() && next.After(*reminder.TTL) {
		return next, "", false
	}

	// Append the execution count if we are tracking repetitions
	if repeats > 0 {
		updatedPeriod += "||" + strconv.Itoa(executedCount)
	}

	return next, updatedPeriod, true
}
