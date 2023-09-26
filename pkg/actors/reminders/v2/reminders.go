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

package reminders

import (
	"context"

	kclock "k8s.io/utils/clock"

	"github.com/dapr/dapr/pkg/actors/internal"
	"github.com/dapr/dapr/pkg/resiliency"
	"github.com/dapr/kit/logger"
)

var log = logger.NewLogger("dapr.runtime.actor.reminders")

const (
	daprSeparator        = "||"
	metadataPartitionKey = "partitionKey"
)

// Implements a reminders provider.
type reminders struct {
	clock             kclock.WithTicker
	executeReminderFn internal.ExecuteReminderFn
	resiliency        resiliency.Provider
	config            internal.Config
}

// NewRemindersProvider returns a reminders provider.
func NewRemindersProvider(clock kclock.WithTicker, opts internal.RemindersProviderOpts) internal.RemindersProvider {
	return &reminders{
		clock:  clock,
		config: opts.Config,
	}
}

func (r *reminders) SetExecuteReminderFn(fn internal.ExecuteReminderFn) {
	r.executeReminderFn = fn
}

func (r *reminders) SetResiliencyProvider(resiliency resiliency.Provider) {
	r.resiliency = resiliency
}

func (r *reminders) CreateReminder(ctx context.Context, reminder *internal.Reminder) error {
	panic("unimplemented")
}

func (r *reminders) Close() error {
	return nil
}

func (r *reminders) Init(ctx context.Context) error {
	return nil
}

func (r *reminders) GetReminder(ctx context.Context, req *internal.GetReminderRequest) (*internal.Reminder, error) {
	panic("unimplemented")
}

func (r *reminders) DeleteReminder(ctx context.Context, req internal.DeleteReminderRequest) error {
	panic("unimplemented")
}

func (r *reminders) SetStateStoreProviderFn(fn internal.StateStoreProviderFn) {
	// SetStateStoreProviderFn is a no-op in this implementation.
}

func (r *reminders) SetLookupActorFn(fn internal.LookupActorFn) {
	// SetLookupActorFn is a no-op in this implementation.
}

func (r *reminders) OnPlacementTablesUpdated(ctx context.Context) {
	// OnPlacementTablesUpdated is a no-op in this implementation.
}

func (r *reminders) DrainRebalancedReminders(actorType string, actorID string) {
	// DrainRebalancedReminders is a no-op in this implementation.
}
