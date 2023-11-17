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
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slices"
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
	GetAllHosts() (TestDataHosts, error)

	// GetAllReminders returns the entire list of reminders in the database.
	GetAllReminders() (TestDataReminders, error)

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
	Hosts     TestDataHosts
	Reminders TestDataReminders
}

type TestDataHosts map[string]TestDataHost

// String implements fmt.Stringer and is useful for debugging.
func (t TestDataHosts) String() string {
	s := make([]string, len(t))
	i := 0
	for k, v := range t {
		s[i] = fmt.Sprintf("Host [id='%s' %v]", k, v)
		i++
	}
	slices.Sort(s)
	return strings.Join(s, "\n")
}

type TestDataReminders map[string]TestDataReminder

// String implements fmt.Stringer and is useful for debugging.
func (t TestDataReminders) String() string {
	s := make([]string, len(t))
	i := 0
	for k, v := range t {
		s[i] = fmt.Sprintf("Reminder [%s]", v.stringWithID(k))
		i++
	}
	slices.Sort(s)
	return strings.Join(s, "\n")
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

// String implements fmt.Stringer and is useful for debugging.
func (t TestDataHost) String() string {
	return fmt.Sprintf("host[address='%s' appID='%s' apiLevel='%d' lastHealthCheck='%v' actorTypes=[%v]]", t.Address, t.AppID, t.APILevel, t.LastHealthCheck, t.ActorTypes)
}

type TestDataActorType struct {
	IdleTimeout              time.Duration
	ActorIDs                 []string
	ConcurrentRemindersLimit int
}

// String implements fmt.Stringer and is useful for debugging.
func (t TestDataActorType) String() string {
	actorIDs := strings.Join(t.ActorIDs, ",")
	return fmt.Sprintf("actorType[actorIDs=[%s] idleTimeout='%v' concurrentRemindersLimit='%d']", actorIDs, t.IdleTimeout, t.ConcurrentRemindersLimit)
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

// String implements fmt.Stringer and is useful for debugging.
func (t TestDataReminder) String() string {
	return t.stringWithID("")
}

func (t TestDataReminder) stringWithID(id string) string {
	if id != "" {
		id = " id='" + id + "'"
	}
	lease := "nil"
	if t.LeaseID != nil {
		lease = fmt.Sprintf("[id='%s' time='%v' pid='%s']", *t.LeaseID, *t.LeaseTime, *t.LeasePID)
	}
	return fmt.Sprintf("name='%s||%s||%s'%s executionTime='%v' lease=%v", t.ActorType, t.ActorID, t.Name, id, t.ExecutionTime, lease)
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
