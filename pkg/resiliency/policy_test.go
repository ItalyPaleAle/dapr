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

package resiliency

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapr/dapr/pkg/resiliency/breaker"
	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/retry"
)

var testLog = logger.NewLogger("dapr.resiliency.test")

func TestPolicy(t *testing.T) {
	retryValue := retry.DefaultConfig()
	cbValue := breaker.CircuitBreaker{
		Name:     "test",
		Interval: 10 * time.Millisecond,
		Timeout:  10 * time.Millisecond,
	}
	cbValue.Initialize(testLog)
	tests := map[string]struct {
		t  time.Duration
		r  *retry.Config
		cb *breaker.CircuitBreaker
	}{
		"empty": {},
		"all": {
			t:  10 * time.Millisecond,
			r:  &retryValue,
			cb: &cbValue,
		},
	}

	ctx := context.Background()
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			called := atomic.Bool{}
			fn := func(ctx context.Context) (any, error) {
				called.Store(true)
				return nil, nil
			}
			policy := NewRunner[any](ctx, &PolicyDefinition{
				log:  testLog,
				name: name,
				t:    tt.t,
				r:    tt.r,
				cb:   tt.cb,
			})
			policy(fn)
			assert.True(t, called.Load())
		})
	}
}

func TestPolicyTimeout(t *testing.T) {
	tests := []struct {
		name      string
		sleepTime time.Duration
		timeout   time.Duration
		expected  bool
	}{
		{
			name:      "Timeout expires",
			sleepTime: time.Millisecond * 100,
			timeout:   time.Millisecond * 10,
			expected:  false,
		},
		{
			name:      "Timeout OK",
			sleepTime: time.Millisecond * 10,
			timeout:   time.Millisecond * 100,
			expected:  true,
		},
	}

	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			called := atomic.Bool{}
			fn := func(ctx context.Context) (any, error) {
				time.Sleep(test.sleepTime)
				called.Store(true)
				return nil, nil
			}

			policy := NewRunner[any](context.Background(), &PolicyDefinition{
				log:  testLog,
				name: "timeout",
				t:    test.timeout,
			})
			policy(fn)

			assert.Equal(t, test.expected, called.Load())
		})
	}
}

func TestPolicyRetry(t *testing.T) {
	tests := []struct {
		name       string
		maxCalls   int32
		maxRetries int64
		expected   int32
	}{
		{
			name:       "Retries succeed",
			maxCalls:   5,
			maxRetries: 6,
			expected:   6,
		},
		{
			name:       "Retries fail",
			maxCalls:   5,
			maxRetries: 2,
			expected:   3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := atomic.Int32{}
			maxCalls := test.maxCalls
			fn := func(ctx context.Context) (any, error) {
				v := called.Add(1)
				if v <= maxCalls {
					return nil, fmt.Errorf("called (%d) vs Max (%d)", v-1, maxCalls)
				}
				return nil, nil
			}

			policy := NewRunner[any](context.Background(), &PolicyDefinition{
				log:  testLog,
				name: "retry",
				t:    10 * time.Millisecond,
				r:    &retry.Config{MaxRetries: test.maxRetries},
			})
			policy(fn)
			assert.Equal(t, test.expected, called.Load())
		})
	}
}

func BenchmarkNewRunner(b *testing.B) {
	ctx := context.Background()
	log := logger.NewLogger("bench")
	var (
		runner Runner[int]
		err    error
	)
	oper := func(ctx context.Context) (int, error) {
		return 42, nil
	}
	def := &PolicyDefinition{
		log:  log,
		name: "testop",
	}
	for n := 0; n < b.N; n++ {
		runner = NewRunner[int](ctx, def)
		_, err = runner(oper)
		if err != nil {
			b.Fatal("err is not nil")
		}
	}
}

func BenchmarkPolicy(b *testing.B) {
	ctx := context.Background()
	log := logger.NewLogger("bench")
	var (
		runner RunnerAny
		err    error
	)
	oper := func(ctx context.Context) (any, error) {
		return 42, nil
	}
	for n := 0; n < b.N; n++ {
		runner = Policy(ctx, log, "testop", 0, nil, nil)
		_, err = runner(oper)
		if err != nil {
			b.Fatal("err is not nil")
		}
	}
}

func BenchmarkPolicyCasting(b *testing.B) {
	ctx := context.Background()
	log := logger.NewLogger("bench")
	var (
		runner RunnerAny
		resAny any
		res    int
		err    error
	)
	oper := func(ctx context.Context) (any, error) {
		return 42, nil
	}
	for n := 0; n < b.N; n++ {
		runner = Policy(ctx, log, "testop", 0, nil, nil)
		resAny, err = runner(oper)
		if err != nil {
			b.Fatal("err is not nil")
		}
		res, _ = resAny.(int)
		if res != 42 {
			b.Fatal("res is not 42")
		}
	}
}
