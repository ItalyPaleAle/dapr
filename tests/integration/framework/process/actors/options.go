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

package actors

import (
	"time"

	"github.com/dapr/dapr/tests/integration/framework/process/exec"
)

// Option is a function that configures the process.
type Option func(*options)

// options contains the options for running Daprd in integration tests.
type options struct {
	execOpts []exec.Option

	port                         int
	healthzPort                  int
	metricsPort                  int
	hostHealthcheckInterval      time.Duration
	noReminders                  bool
	remindersPollInterval        time.Duration
	remindersFetchAheadInterval  time.Duration
	remindersFetchAheadBatchSize int
	remindersLeaseDuration       time.Duration
	logLevel                     string
	mode                         string
	enableMTLS                   bool
	sentryAddress                string
}

func WithExecOptions(execOptions ...exec.Option) Option {
	return func(o *options) {
		o.execOpts = execOptions
	}
}

func WithPort(port int) Option {
	return func(o *options) {
		o.port = port
	}
}

func WithLogLevel(logLevel string) Option {
	return func(o *options) {
		o.logLevel = logLevel
	}
}

func WithMode(mode string) Option {
	return func(o *options) {
		o.mode = mode
	}
}

func WithEnableMTLS(enable bool) Option {
	return func(o *options) {
		o.enableMTLS = enable
	}
}

func WithSentryAddress(address string) Option {
	return func(o *options) {
		o.sentryAddress = address
	}
}

func WithEnableReminders(enable bool) Option {
	return func(o *options) {
		o.noReminders = !enable
	}
}

func WithHostHealthcheckInterval(val time.Duration) Option {
	return func(o *options) {
		o.hostHealthcheckInterval = val
	}
}

func WithRemindersPollInterval(val time.Duration) Option {
	return func(o *options) {
		o.remindersPollInterval = val
	}
}

func WithRemindersFetchAheadInterval(val time.Duration) Option {
	return func(o *options) {
		o.remindersFetchAheadInterval = val
	}
}

func WithRemindersLeaseDuration(val time.Duration) Option {
	return func(o *options) {
		o.remindersLeaseDuration = val
	}
}

func WithRemindersFetchAheadBatchSize(val int) Option {
	return func(o *options) {
		o.remindersFetchAheadBatchSize = val
	}
}
