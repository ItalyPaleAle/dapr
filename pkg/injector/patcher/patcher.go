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

package patcher

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cast"

	"github.com/dapr/dapr/utils"
)

const (
	PatchPathContainers = "/spec/containers"
	PatchPathVolumes    = "/spec/volumes"
)

// PatchOperation represents a discreet change to be applied to a Kubernetes resource.
type PatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// PodPatcher contains methods to patch a pod to inject Dapr and perform other relevant changes.
// Its parameters can be read from annotations on a pod.
// Note: make sure that the annotations defined here are in-sync with the constants in the annotations package.
type PodPatcher struct {
	Enabled                             bool   `annotation:"dapr.io/enabled"`
	AppPort                             int32  `annotation:"dapr.io/app-port"`
	Config                              string `annotation:"dapr.io/config"`
	AppProtocol                         string `annotation:"dapr.io/app-protocol" default:"http"`
	AppSSL                              bool   `annotation:"dapr.io/app-ssl"` // TODO: Deprecated in Dapr 1.11; remove in a future Dapr version
	AppID                               string `annotation:"dapr.io/app-id"`
	EnableProfiling                     bool   `annotation:"dapr.io/enable-profiling"`
	LogLevel                            string `annotation:"dapr.io/log-level" default:"info"`
	APITokenSecret                      string `annotation:"dapr.io/api-token-secret"`
	AppTokenSecret                      string `annotation:"dapr.io/app-token-secret"`
	LogAsJSON                           bool   `annotation:"dapr.io/log-as-json"`
	AppMaxConcurrency                   int32  `annotation:"dapr.io/app-max-concurrency"`
	EnableMetrics                       bool   `annotation:"dapr.io/enable-metrics" default:"true"`
	MetricsPort                         int32  `annotation:"dapr.io/metrics-port" default:"9090"`
	EnableDebug                         bool   `annotation:"dapr.io/enable-debug"`
	DebugPort                           int32  `annotation:"dapr.io/debug-port" default:"40000"`
	Env                                 string `annotation:"dapr.io/env"`
	SidecarCPURequest                   string `annotation:"dapr.io/sidecar-cpu-request"`
	SidecarCPULimit                     string `annotation:"dapr.io/sidecar-cpu-limit"`
	SidecarMemoryRequest                string `annotation:"dapr.io/sidecar-memory-request"`
	SidecarMemoryLimit                  string `annotation:"dapr.io/sidecar-memory-limit"`
	SidecarListenAddresses              string `annotation:"dapr.io/sidecar-listen-addresses" default:"[::1],127.0.0.1"`
	SidecarLivenessProbeDelaySeconds    int32  `annotation:"dapr.io/sidecar-liveness-probe-delay-seconds" default:"3"`
	SidecarLivenessProbeTimeoutSeconds  int32  `annotation:"dapr.io/sidecar-liveness-probe-timeout-seconds" default:"3"`
	SidecarLivenessProbePeriodSeconds   int32  `annotation:"dapr.io/sidecar-liveness-probe-period-seconds" default:"6"`
	SidecarLivenessProbeThreshold       int32  `annotation:"dapr.io/sidecar-liveness-probe-threshold" default:"3"`
	SidecarReadinessProbeDelaySeconds   int32  `annotation:"dapr.io/sidecar-readiness-probe-delay-seconds" default:"3"`
	SidecarReadinessProbeTimeoutSeconds int32  `annotation:"dapr.io/sidecar-readiness-probe-timeout-seconds" default:"3"`
	SidecarReadinessProbePeriodSeconds  int32  `annotation:"dapr.io/sidecar-readiness-probe-period-seconds" default:"6"`
	SidecarReadinessProbeThreshold      int32  `annotation:"dapr.io/sidecar-readiness-probe-threshold" default:"3"`
	SidecarImage                        string `annotation:"dapr.io/sidecar-image"`
	SidecarSeccompProfileTyle           string `annotation:"dapr.io/sidecar-seccomp-profile-type"`
	HTTPMaxRequestSize                  int32  `annotation:"dapr.io/http-max-request-size"`
	HTTPReadBufferSize                  int32  `annotation:"dapr.io/http-read-buffer-size"`
	GracefulShutdownSeconds             int32  `annotation:"dapr.io/graceful-shutdown-seconds"`
	EnableAPILogging                    bool   `annotation:"dapr.io/enable-api-logging"`
	UnixDomainSocketPath                string `annotation:"dapr.io/unix-domain-socket-path"`
	VolumeMounts                        string `annotation:"dapr.io/volume-mounts"`
	VolumeMountsRW                      string `annotation:"dapr.io/volume-mounts-rw"`
	DisableBuiltinK8sSecretStore        bool   `annotation:"dapr.io/disable-builtin-k8s-secret-store"`
	EnableAppHealthCheck                bool   `annotation:"dapr.io/enable-app-health-check"`
	AppHealthCheckPath                  string `annotation:"dapr.io/app-health-check-path" default:"/healthz"`
	AppHealthProbeInterval              int32  `annotation:"dapr.io/app-health-probe-interval" default:"5"`  // In seconds
	AppHealthProbeTimeout               int32  `annotation:"dapr.io/app-health-probe-timeout" default:"500"` // In milliseconds
	AppHealthThreshold                  int32  `annotation:"dapr.io/app-health-threshold" default:"3"`
	PlacementHostAddress                string `annotation:"dapr.io/placement-host-address"`
	PluggableComponents                 string `annotation:"dapr.io/pluggable-components"`
	PluggableComponentsSocketsFolder    string `annotation:"dapr.io/pluggable-components-sockets-folder"`
	ComponentContainer                  string `annotation:"dapr.io/component-container"`
	InjectPluggableComponents           bool   `annotation:"dapr.io/inject-pluggable-components"`
	AppChannelAddress                   string `annotation:"dapr.io/app-channel-address"`
}

// NewPodPatcher returns a PodPatcher object with the default values set.
func NewPodPatcher() *PodPatcher {
	p := &PodPatcher{}
	p.setDefaultValues()
	return p
}

func (p *PodPatcher) setDefaultValues() {
	// Iterate through the fields using reflection
	val := reflect.ValueOf(p).Elem()
	for i := 0; i < val.NumField(); i++ {
		fieldT := val.Type().Field(i)
		fieldV := val.Field(i)
		an := fieldT.Tag.Get("annotation")
		def := fieldT.Tag.Get("default")
		if !fieldV.CanSet() || an == "" || def == "" {
			continue
		}

		// Assign the default value
		setValueFromString(fieldT.Type.Kind(), fieldV, def)
	}
}

// SetFromAnnotations updates the object with properties from an annotation map.
func (p *PodPatcher) SetFromAnnotations(an map[string]string) {
	// Iterate through the fields using reflection
	val := reflect.ValueOf(p).Elem()
	for i := 0; i < val.NumField(); i++ {
		fieldV := val.Field(i)
		fieldT := val.Type().Field(i)
		key := fieldT.Tag.Get("annotation")
		if !fieldV.CanSet() || key == "" {
			continue
		}

		// Skip annotations that are not defined or which have an empty value
		if an[key] == "" {
			continue
		}

		// Assign the value
		setValueFromString(fieldT.Type.Kind(), fieldV, an[key])
	}
}

func setValueFromString(rk reflect.Kind, rv reflect.Value, val string) {
	switch rk {
	case reflect.String:
		rv.SetString(val)
	case reflect.Bool:
		rv.SetBool(utils.IsTruthy(val))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, err := strconv.ParseInt(val, 10, 64)
		if err == nil {
			rv.SetInt(v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v, err := strconv.ParseUint(val, 10, 64)
		if err == nil {
			rv.SetUint(v)
		}
	}
}

// String implements fmt.Stringer and is used to print the state of the object, primarily for debugging purposes.
func (p *PodPatcher) String() string {
	return p.toString(false)
}

// StringAll returns the list of all annotations (including empty ones), primarily for debugging purposes.
func (p *PodPatcher) StringAll() string {
	return p.toString(true)
}

func (p *PodPatcher) toString(includeAll bool) string {
	res := strings.Builder{}

	// Iterate through the fields using reflection
	val := reflect.ValueOf(p).Elem()
	for i := 0; i < val.NumField(); i++ {
		fieldT := val.Type().Field(i)
		fieldV := val.Field(i)
		key := fieldT.Tag.Get("annotation")
		def := fieldT.Tag.Get("default")
		if key == "" {
			continue
		}

		// Do not print default values or zero values when there's no default
		val := cast.ToString(fieldV.Interface())
		if includeAll || (def != "" && val != def) || (def == "" && !fieldV.IsZero()) {
			res.WriteString(key + ": " + strconv.Quote(val) + "\n")
		}
	}

	return res.String()
}
