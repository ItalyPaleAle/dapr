/*
Copyright 2021 The Dapr Authors
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

package apphealth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestAppHealth_ratelimitReports(t *testing.T) {
	// Set to 0.1 seconds
	var minInterval int64 = 1e5

	h := &AppHealth{
		lastReport:        atomic.NewInt64(0),
		reportMinInterval: minInterval,
	}

	// First run should always suceed
	require.True(t, h.ratelimitReports())

	// Run again without waiting
	require.False(t, h.ratelimitReports())
	require.False(t, h.ratelimitReports())

	// Wait and test
	time.Sleep(time.Duration(minInterval+10) * time.Microsecond)
	require.True(t, h.ratelimitReports())
	require.False(t, h.ratelimitReports())

	// Run tests for 1 second, constantly
	// Should succeed only 10 times (+/- 1)
	time.Sleep(time.Duration(minInterval+10) * time.Microsecond)
	firehose := func() (passed int64) {
		var done bool
		deadline := time.After(time.Second)
		for !done {
			select {
			case <-deadline:
				done = true
			default:
				if h.ratelimitReports() {
					passed++
				}
				time.Sleep(10 * time.Nanosecond)
			}
		}

		return passed
	}

	passed := firehose()

	assert.GreaterOrEqual(t, passed, int64(9))
	assert.LessOrEqual(t, passed, int64(11))

	// Repeat, but run with 3 parallel goroutines
	time.Sleep(time.Duration(minInterval+10) * time.Microsecond)
	wg := sync.WaitGroup{}
	totalPassed := atomic.NewInt64(0)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			passed := firehose()
			totalPassed.Add(passed)
			wg.Done()
		}()
	}
	wg.Wait()

	passed = totalPassed.Load()
	assert.GreaterOrEqual(t, passed, int64(9))
	assert.LessOrEqual(t, passed, int64(11))
}
