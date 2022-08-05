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
	"context"
	"time"

	"go.uber.org/atomic"

	"github.com/dapr/kit/logger"
)

const (
	AppStatusUnhealthy uint8 = 0
	AppStatusHealthy   uint8 = 1
)

var log = logger.NewLogger("dapr.apphealth")

// AppHealth manages the health checks for the app.
type AppHealth struct {
	config       *Config
	probeFn      ProbeFunction
	changeCb     ChangeCallback
	report       chan uint8
	failureCount *atomic.Int32
	lastReport   *atomic.Int64
}

// ProbeFunction is the signature of the function that performs health probes.
// Health probe functions return errors only in case of internal errors.
// Network errors are considered probe failures, and should return nil as errors.
type ProbeFunction func(context.Context) (bool, error)

// ChangeCallback is the signature of the callback that is invoked when the app's health status changes.
type ChangeCallback func(status uint8)

// NewAppHealth creates a new AppHealth object.
func NewAppHealth(config *Config, probeFn ProbeFunction) *AppHealth {
	return &AppHealth{
		config:       config,
		probeFn:      probeFn,
		report:       make(chan uint8, 1),
		failureCount: atomic.NewInt32(0),
		lastReport:   atomic.NewInt64(0),
	}
}

// OnHealthChange sets the callback that is invoked when the health of the app changes (app becomes either healthy or unhealthy).
func (h *AppHealth) OnHealthChange(cb ChangeCallback) {
	h.changeCb = cb
}

// StartProbes starts polling the app on the interval.
func (h *AppHealth) StartProbes(ctx context.Context) {
	if h.probeFn == nil {
		log.Fatal("Cannot start probes with nil probe function")
	}
	if h.config.ProbeTimeout > h.config.ProbeInterval {
		log.Fatal("App health checks probe timeouts must be smaller than probe intervals")
	}

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
				go h.doProbe(ctx)
			case status := <-h.report:
				log.Debug("Received health status report")
				ticker.Reset(h.config.ProbeInterval)
				h.setResult(status == AppStatusHealthy)
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

// Performs a health probe.
// Should be invoked in a background goroutine.
func (h *AppHealth) doProbe(parentCtx context.Context) {
	ctx, cancel := context.WithTimeout(parentCtx, h.config.ProbeTimeout)
	successful, err := h.probeFn(ctx)
	cancel()

	// In case of errors, we do not record the failed probe because this is generally an internal error
	if err != nil {
		log.Errorf("App health probe could not complete with error: %v", err)
		return
	}

	h.setResult(successful)
}

// Returns true if the health report can be saved. Only 1 report per second at most is allowed.
func (h *AppHealth) ratelimitReports() bool {
	var (
		swapped  bool
		attempts uint8
	)

	now := time.Now().UnixMicro()

	// Attempts at most 2 times before giving up, as the report may be stale at that point
	for !swapped && attempts < 2 {
		attempts++

		// If the last report was less than 1 second ago, nothing to do here
		prev := h.lastReport.Load()
		if prev > now-10e6 {
			return false
		}

		swapped = h.lastReport.CAS(prev, now)
	}

	// If we couldn't do the swap after 2 attempts, just return false
	return swapped
}

func (h *AppHealth) setResult(successful bool) {
	h.lastReport.Store(time.Now().UnixMicro())

	if successful {
		// Reset the failure count
		// If the previous value was >= threshold, we need to report a health change
		prev := h.failureCount.Swap(0)
		if prev >= h.config.Threshold && h.changeCb != nil {
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
