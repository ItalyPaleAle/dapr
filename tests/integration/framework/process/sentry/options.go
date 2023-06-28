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

package sentry

import (
	"github.com/dapr/dapr/pkg/sentry/server/ca"
	"github.com/dapr/dapr/tests/integration/framework/process/exec"
)

// options contains the options for running Sentry in integration tests.
type options struct {
	execOpts []exec.Option

	ca          *certs.Credentials
	rootPEM     []byte
	certPEM     []byte
	keyPEM      []byte
	port        int
	healthzPort int
	metricsPort int
}

// Option is a function that configures the process.
type Option func(*options)

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

func WithMetricsPort(port int) Option {
	return func(o *options) {
		o.metricsPort = port
	}
}

func WithHealthzPort(port int) Option {
	return func(o *options) {
		o.healthzPort = port
	}
}

func WithCABundle(bundle ca.CABundle) Option {
	return func(o *options) {
		o.bundle = bundle
	}
}
