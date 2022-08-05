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
	"time"
)

const (
	// DefaultAppCheckPath is the default path for HTTP health checks.
	DefaultAppCheckPath = "/health"
	// DefaultAppHealthProbeInterval is the default interval for app health probes.
	DefaultAppHealthProbeInterval = time.Second * 5
	// DefaultAppHealthThreshold is the default threshold for determining failures in app health checks.
	DefaultAppHealthThreshold = 3

	AppStatusUnhealthy = 0
	AppStatusHealthy   = 1
)

// AppHealthConfig is the configuration object for the app health checks.
type AppHealthConfig struct {
	HTTPPath      string
	ProbeInterval time.Duration
	ProbeOnly     bool
	Threshold     int
}

// AppHealth manages the health checks for the app.
type AppHealth struct {
	changeCb func(int)
}

// OnHealthChange sets the callback that is invoked when the health of the app changes (app becomes either healthy or unhealthy).
func (h *AppHealth) OnHealthChange(cb func(status int)) {
	h.changeCb = cb
}
