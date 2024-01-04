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

package emmy

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapr/dapr/tests/integration/framework/binary"
	"github.com/dapr/dapr/tests/integration/framework/process"
	"github.com/dapr/dapr/tests/integration/framework/process/exec"
	"github.com/dapr/dapr/tests/integration/framework/util"
)

type Emmy struct {
	exec     process.Interface
	freeport *util.FreePort

	port        int
	metricsPort int
	healthzPort int
}

func New(t *testing.T, fopts ...Option) *Emmy {
	t.Helper()

	fp := util.ReservePorts(t, 3)
	opts := options{
		logLevel:    "info",
		port:        fp.Port(t, 0),
		metricsPort: fp.Port(t, 1),
		healthzPort: fp.Port(t, 2),
		mode:        "standalone",
		storeName:   "sqlite",
		storeOpts:   map[string]string{"connectionString": "file::memory:"},
	}

	for _, fopt := range fopts {
		fopt(&opts)
	}

	args := []string{
		"--log-level=" + opts.logLevel,
		"--port=" + strconv.Itoa(opts.port),
		"--healthz-port=" + strconv.Itoa(opts.healthzPort),
		"--metrics-port=" + strconv.Itoa(opts.metricsPort),
		"--store-name=sqlite",
		"--mode=" + opts.mode,
		"--enable-mtls=" + strconv.FormatBool(opts.enableMTLS),
	}

	if opts.hostHealthcheckInterval > 0 {
		args = append(args, "--host-healthcheck-interval="+opts.hostHealthcheckInterval.String())
	}
	if opts.noReminders {
		args = append(args, "--no-reminders")
	}
	if opts.remindersPollInterval > 0 {
		args = append(args, "--reminders-poll-interval="+opts.remindersPollInterval.String())
	}
	if opts.remindersFetchAheadInterval > 0 {
		args = append(args, "--reminders-fetch-ahead-interval="+opts.remindersFetchAheadInterval.String())
	}
	if opts.remindersFetchAheadBatchSize > 0 {
		args = append(args, "--reminders-fetch-ahead-batch-size="+strconv.Itoa(opts.remindersFetchAheadBatchSize))
	}
	if opts.remindersLeaseDuration > 0 {
		args = append(args, "--reminders-lease-duration="+opts.remindersLeaseDuration.String())
	}
	if len(opts.sentryAddress) > 0 {
		args = append(args, "--sentry-address="+opts.sentryAddress)
	}

	return &Emmy{
		exec: exec.New(t,
			binary.EnvValue("emmy"), args,
			opts.execOpts...,
		),
		freeport:    fp,
		port:        opts.port,
		metricsPort: opts.metricsPort,
		healthzPort: opts.healthzPort,
	}
}

func (o *Emmy) Run(t *testing.T, ctx context.Context) {
	o.freeport.Free(t)
	o.exec.Run(t, ctx)
}

func (o *Emmy) Cleanup(t *testing.T) {
	o.exec.Cleanup(t)
}

func (o *Emmy) WaitUntilRunning(t *testing.T, ctx context.Context) {
	client := util.HTTPClient(t)
	assert.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://localhost:%d/healthz", o.healthzPort), nil)
		if err != nil {
			return false
		}
		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return http.StatusOK == resp.StatusCode
	}, time.Second*5, 100*time.Millisecond)
}

func (o *Emmy) Port() int {
	return o.port
}

func (o *Emmy) Address() string {
	return "localhost:" + strconv.Itoa(o.port)
}

func (o *Emmy) MetricsPort() int {
	return o.metricsPort
}

func (o *Emmy) HealthzPort() int {
	return o.healthzPort
}
