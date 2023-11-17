//go:build conftests || e2e

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

package actorstore

import (
	"time"
)

// StoreConfTests is defined here to include the methods that are to be implemented for conftests.
// This file has the "conftests" build tag.
// The same is defined in actorstore_no_conftests.go, where there's no build tag, as an empty interface.
type StoreConfTests interface {
	// SetupConformanceTests performs the setup of test resources.
	SetupConformanceTests() error

	// CleanupConformanceTests performs the cleanup of test resources.
	CleanupConformanceTests() error

	// GetAllHosts returns the entire list of hosts in the database.
	GetAllHosts() (map[string]TestDataHost, error)

	// GetAllReminders returns the entire list of reminders in the database.
	GetAllReminders() (map[string]TestDataReminder, error)

	// LoadActorStateTestData loads all actor state test data in the databas
	LoadActorStateTestData(testData TestData) error

	// LoadReminderTestData loads all reminder test data in the database.
	LoadReminderTestData(testData TestData) error

	// GetTime returns the current time.
	// Implementations that do not support mocking of time should return time.Now().
	GetTime() time.Time

	// AdvanceTime makes the time advance by the specified duration.
	// Implementations that support mocking of time can use this to advance the internal clock.
	// Other implementations, who can't rely on time mocking, should implement this with a `time.Sleep(d)`.
	AdvanceTime(d time.Duration) error
}

type TestData struct {
	Hosts     map[string]TestDataHost
	Reminders map[string]TestDataReminder
}

type TestDataHost struct {
	Address              string
	AppID                string
	APILevel             int
	LastHealthCheck      time.Time
	LastHealthCheckStore time.Duration
	ActorTypes           map[string]TestDataActorType
}

func (t TestDataHost) IsActive(now time.Time, hostHealthCheckInterval time.Duration) bool {
	return now.Sub(t.LastHealthCheck) < hostHealthCheckInterval
}

type TestDataActorType struct {
	IdleTimeout              time.Duration
	ActorIDs                 []string
	ConcurrentRemindersLimit int
}

type TestDataReminder struct {
	ActorType     string
	ActorID       string
	Name          string
	ExecutionTime time.Duration // Interval from current time
	LeaseID       *string
	LeaseTime     *time.Time
	LeasePID      *string
}

func (t TestData) HostsByActorType(now time.Time, hostHealthCheckInterval time.Duration) map[string][]string {
	res := make(map[string][]string)
	for hostID, host := range t.Hosts {
		for at := range host.ActorTypes {
			if !host.IsActive(now, hostHealthCheckInterval) {
				continue
			}

			if res[at] == nil {
				res[at] = []string{hostID}
			} else {
				res[at] = append(res[at], hostID)
			}
		}
	}
	return res
}

func (t TestData) HostsForActorType(now time.Time, actorType string, hostHealthCheckInterval time.Duration) []string {
	res := make([]string, 0)
	for hostID, host := range t.Hosts {
		if !host.IsActive(now, hostHealthCheckInterval) {
			continue
		}

		_, ok := host.ActorTypes[actorType]
		if ok {
			res = append(res, hostID)
		}
	}
	return res
}
