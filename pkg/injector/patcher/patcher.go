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
	AppProtocol                         string `annotation:"dapr.io/app-protocol"`
	AppSSL                              bool   `annotation:"dapr.io/app-ssl"` // TODO: Deprecated in Dapr 1.11; remove in a future Dapr version
	AppID                               string `annotation:"dapr.io/app-id"`
	EnableProfiling                     bool   `annotation:"dapr.io/enable-profiling"`
	LogLevel                            string `annotation:"dapr.io/log-level"`
	APITokenSecret                      string `annotation:"dapr.io/api-token-secret"`
	AppTokenSecret                      string `annotation:"dapr.io/app-token-secret"`
	LogAsJSON                           bool   `annotation:"dapr.io/log-as-json"`
	AppMaxConcurrency                   int32  `annotation:"dapr.io/app-max-concurrency"`
	EnableMetrics                       bool   `annotation:"dapr.io/enable-metrics"`
	MetricsPort                         int32  `annotation:"dapr.io/metrics-port"`
	EnableDebug                         bool   `annotation:"dapr.io/enable-debug"`
	DebugPort                           int32  `annotation:"dapr.io/debug-port"`
	Env                                 string `annotation:"dapr.io/env"`
	SidecarCPURequest                   string `annotation:"dapr.io/sidecar-cpu-request"`
	SidecarCPULimit                     string `annotation:"dapr.io/sidecar-cpu-limit"`
	SidecarMemoryRequest                string `annotation:"dapr.io/sidecar-memory-request"`
	SidecarMemoryLimit                  string `annotation:"dapr.io/sidecar-memory-limit"`
	SidecarListenAddresses              string `annotation:"dapr.io/sidecar-listen-addresses"`
	SidecarLivenessProbeDelaySeconds    int32  `annotation:"dapr.io/sidecar-liveness-probe-delay-seconds"`
	SidecarLivenessProbeTimeoutSeconds  int32  `annotation:"dapr.io/sidecar-liveness-probe-timeout-seconds"`
	SidecarLivenessProbePeriodSeconds   int32  `annotation:"dapr.io/sidecar-liveness-probe-period-seconds"`
	SidecarLivenessProbeThreshold       int32  `annotation:"dapr.io/sidecar-liveness-probe-threshold"`
	SidecarReadinessProbeDelaySeconds   int32  `annotation:"dapr.io/sidecar-readiness-probe-delay-seconds"`
	SidecarReadinessProbeTimeoutSeconds int32  `annotation:"dapr.io/sidecar-readiness-probe-timeout-seconds"`
	SidecarReadinessProbePeriodSeconds  int32  `annotation:"dapr.io/sidecar-readiness-probe-period-seconds"`
	SidecarReadinessProbeThreshold      int32  `annotation:"dapr.io/sidecar-readiness-probe-threshold"`
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
	AppHealthCheckPath                  string `annotation:"dapr.io/app-health-check-path"`
	AppHealthProbeInterval              int32  `annotation:"dapr.io/app-health-probe-interval"` // In seconds
	AppHealthProbeTimeout               int32  `annotation:"dapr.io/app-health-probe-timeout"`  // In milliseconds
	AppHealthThreshold                  int32  `annotation:"dapr.io/app-health-threshold"`
	PlacementHostAddress                string `annotation:"dapr.io/placement-host-address"`
	PluggableComponents                 string `annotation:"dapr.io/pluggable-components"`
	PluggableComponentsSocketsFolder    string `annotation:"dapr.io/pluggable-components-sockets-folder"`
	ComponentContainer                  string `annotation:"dapr.io/component-container"`
	InjectPluggableComponents           bool   `annotation:"dapr.io/inject-pluggable-components"`
	AppChannelAddress                   string `annotation:"dapr.io/app-channel-address"`
}

// NewPodPatcher returns a PodPatcher object with the default values set.
func NewPodPatcher() *PodPatcher {
	return &PodPatcher{
		LogLevel:                            "info",
		LogAsJSON:                           false,
		AppSSL:                              false,
		AppProtocol:                         "http",
		EnableMetrics:                       true,
		MetricsPort:                         9090,
		EnableDebug:                         false,
		DebugPort:                           40000,
		SidecarListenAddresses:              "[::1],127.0.0.1",
		SidecarLivenessProbeDelaySeconds:    3,
		SidecarLivenessProbeTimeoutSeconds:  3,
		SidecarLivenessProbePeriodSeconds:   6,
		SidecarLivenessProbeThreshold:       3,
		SidecarReadinessProbeDelaySeconds:   3,
		SidecarReadinessProbeTimeoutSeconds: 3,
		SidecarReadinessProbePeriodSeconds:  6,
		SidecarReadinessProbeThreshold:      3,
		EnableProfiling:                     false,
		DisableBuiltinK8sSecretStore:        false,
		EnableAppHealthCheck:                false,
		AppHealthCheckPath:                  "/healthz",
		AppHealthProbeInterval:              5,
		AppHealthProbeTimeout:               500,
		AppHealthThreshold:                  3,
	}
}

// SetFromAnnotations updates the object with properties from an annotation map.
func (p *PodPatcher) SetFromAnnotations(an map[string]string) {
	// Iterate through the fields using reflection
	val := reflect.ValueOf(p).Elem()
	for i := 0; i < val.NumField(); i++ {
		fieldT := val.Type().Field(i)
		key := fieldT.Tag.Get("annotation")
		if key == "" {
			continue
		}

		fieldV := val.Field(i)
		if !fieldV.CanSet() {
			continue
		}

		// Skip annotations that are not defined or which have an empty value
		if an[key] == "" {
			continue
		}

		// Assign the value
		switch fieldT.Type.Kind() {
		case reflect.String:
			fieldV.SetString(an[key])
		case reflect.Bool:
			fieldV.SetBool(utils.IsTruthy(an[key]))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v, err := strconv.ParseInt(an[key], 10, 64)
			if err == nil {
				fieldV.SetInt(v)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v, err := strconv.ParseUint(an[key], 10, 64)
			if err == nil {
				fieldV.SetUint(v)
			}
		}
	}
}

// String implements fmt.Stringer and is used to print the state of the object, primarily for debugging purposes.
func (p *PodPatcher) String() string {
	res := strings.Builder{}

	// Iterate through the fields using reflection
	val := reflect.ValueOf(p).Elem()
	for i := 0; i < val.NumField(); i++ {
		fieldT := val.Type().Field(i)
		key := fieldT.Tag.Get("annotation")
		if key == "" {
			continue
		}

		fieldV := val.Field(i)
		if !fieldV.CanSet() {
			continue
		}

		val := cast.ToString(fieldV.Interface())
		if val != "" {
			res.WriteString(key + ": " + strconv.Quote(val) + "\n")
		}
	}

	return res.String()
}
