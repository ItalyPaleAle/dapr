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
	"fmt"

	"github.com/dapr/components-contrib/actorstore"
	"github.com/dapr/kit/events/queue"
	"github.com/dapr/kit/logger"
)

func (s *server) pollForReminders(ctx context.Context) {
	log.Infof("Start polling for reminder with interval %v", s.opts.RemindersPollInterval)

	s.processor = queue.NewProcessor[*actorstore.FetchedReminder](s.processReminder)
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
		r := reminders[i]
		if log.IsOutputLevelEnabled(logger.DebugLevel) {
			log.Debugf("Scheduling reminder '%s' to be executed at '%v'", r.Key(), r.ScheduledTime())
		}
		err = s.processor.Enqueue(&r)
		if err != nil {
			return fmt.Errorf("failed to enqueue reminder %s: %w", r.Key(), err)
		}
	}

	return nil
}

func (s *server) processReminder(r *actorstore.FetchedReminder) {
	fmt.Println("EXECUTING REMINDER", r.Key())
}

func (s *server) fetchReminders(ctx context.Context) ([]actorstore.FetchedReminder, error) {
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
