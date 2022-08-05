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

package health

import (
	"context"
	"time"

	"github.com/dapr/kit/logger"
	"go.uber.org/atomic"
)

const (
	// DefaultAppCheckPath is the default path for HTTP health checks.
	DefaultAppCheckPath = "/health"
	// DefaultAppHealthProbeInterval is the default interval for app health probes.
	DefaultAppHealthProbeInterval = time.Second * 5
	// DefaultAppHealthThreshold is the default threshold for determining failures in app health checks.
	DefaultAppHealthThreshold = int32(3)

	AppStatusUnhealthy uint8 = 0
	AppStatusHealthy   uint8 = 1
)

var log = logger.NewLogger("dapr.runtime.health")

// AppHealthConfig is the configuration object for the app health checks.
type AppHealthConfig struct {
	HTTPPath      string
	ProbeInterval time.Duration
	ProbeOnly     bool
	Threshold     int32
}

// AppHealth manages the health checks for the app.
type AppHealth struct {
	config       *AppHealthConfig
	changeCb     func(uint8)
	report       chan uint8
	failureCount *atomic.Int32
	lastReport   *atomic.Int64
}

// NewAppHealth creates a new AppHealth object.
func NewAppHealth(config *AppHealthConfig) *AppHealth {
	return &AppHealth{
		config:       config,
		failureCount: atomic.NewInt32(0),
		report:       make(chan uint8, 1),
	}
}

// OnHealthChange sets the callback that is invoked when the health of the app changes (app becomes either healthy or unhealthy).
func (h *AppHealth) OnHealthChange(cb func(status uint8)) {
	h.changeCb = cb
}

// StartProbes starts polling the app on the interval.
func (h *AppHealth) StartProbes(ctx context.Context) {
	log.Info("App health probes starting")

	go func() {
		ticker := time.NewTicker(h.config.ProbeInterval)

		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				log.Info("App health probes stopping")
				return
			case <-ticker.C:
				log.Debug("Probing app health")
			case status := <-h.report:
				log.Debug("Received health status report")
				h.setFailure(status == AppStatusUnhealthy)
			}
		}
	}()
}

// ReportHealth is used by the runtime to report a health signal from the app.
func (h *AppHealth) ReportHealth(status uint8) {
	// If the user wants health probes only, short-circuit here
	if h.config.ProbeOnly {
		return
	}

	// Limit health reports to 1 per second
	if !h.ratelimitReports() {
		return
	}

	// Channel is buffered, so make sure that this doesn't block
	select {
	case h.report <- status:
		// No action
	default:
		// No action
	}
}

// Returns true if the health report can be saved. Only 1 report per second at most is allowed.
func (h *AppHealth) ratelimitReports() bool {
	var (
		swapped  bool
		attempts uint8
	)

	now := time.Now().UnixMicro()

	// Attempts at most 10 times before giving up
	for !swapped && attempts < 10 {
		attempts++

		// If the last report was less than 1 second ago, nothing to do here
		prev := h.lastReport.Load()
		if prev > now-10e6 {
			return false
		}

		swapped = h.lastReport.CAS(prev, now)
	}

	// If we couldn't do the swap after 10 attempts, just return false
	return swapped
}

func (h *AppHealth) setFailure(failed bool) {
	if !failed {
		// Reset the failure count
		// If the previous value was >= threshold, we need to report a health change
		prev := h.failureCount.Swap(0)
		if prev >= int32(h.config.Threshold) && h.changeCb != nil {
			go h.changeCb(AppStatusHealthy)
		}
		return
	}

	// Count the failure
	failures := h.failureCount.Inc()

	// First, check if we've overflown
	if failures < 0 {
		// Reset to the threshold + 1
		h.failureCount.Store(h.config.Threshold + 1)
	} else if failures == h.config.Threshold && h.changeCb != nil {
		// If we're here, we just passed the threshold right now
		go h.changeCb(AppStatusUnhealthy)
	}
}
