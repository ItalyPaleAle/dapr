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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cast"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/dapr/dapr/pkg/components/pluggable"
	"github.com/dapr/dapr/pkg/injector/annotations"
	authConsts "github.com/dapr/dapr/pkg/runtime/security/consts"
	sentryConsts "github.com/dapr/dapr/pkg/sentry/consts"
	"github.com/dapr/dapr/utils"
	"github.com/dapr/kit/logger"
	"github.com/dapr/kit/ptr"
)

var log = logger.NewLogger("dapr.injector.patcher")

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
	Namespace                   string
	CertChain                   string
	CertKey                     string
	ControlPlaneAddress         string
	Identity                    string
	IgnoreEntrypointTolerations []corev1.Toleration
	ImagePullPolicy             corev1.PullPolicy
	MTLSEnabled                 bool
	PlacementServiceAddress     string
	SentryAddress               string
	Tolerations                 []corev1.Toleration
	TrustAnchors                string
	SkipPlacement               bool
	RunAsNonRoot                bool
	ReadOnlyRootFilesystem      bool
	SidecarDropALLCapabilities  bool

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
	EnableAPILogging                    *bool  `annotation:"dapr.io/enable-api-logging"`
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
		setValueFromString(fieldT.Type, fieldV, an[key])
	}
}

// GetSidecarContainer returns the Container object for the sidecar.
func (p *PodPatcher) GetSidecarContainer() (*corev1.Container, error) {
	var appPort string
	if p.AppPort > 0 {
		appPort = strconv.FormatInt(int64(p.AppPort), 10)
	}

	// We still include PlacementServiceAddress if explicitly set
	placementSvcAddress := p.PlacementServiceAddress
	if p.PlacementHostAddress != "" {
		placementSvcAddress = p.PlacementHostAddress
	} else if p.SkipPlacement {
		placementSvcAddress = ""
	}

	ports := []corev1.ContainerPort{
		{
			ContainerPort: int32(SidecarHTTPPort),
			Name:          SidecarHTTPPortName,
		},
		{
			ContainerPort: int32(SidecarAPIGRPCPort),
			Name:          SidecarGRPCPortName,
		},
		{
			ContainerPort: int32(SidecarInternalGRPCPort),
			Name:          SidecarInternalGRPCPortName,
		},
		{
			ContainerPort: p.MetricsPort,
			Name:          SidecarMetricsPortName,
		},
	}

	cmd := []string{"/daprd"}

	args := []string{
		"--mode", "kubernetes",
		"--dapr-http-port", strconv.Itoa(SidecarHTTPPort),
		"--dapr-grpc-port", strconv.Itoa(SidecarAPIGRPCPort),
		"--dapr-internal-grpc-port", strconv.Itoa(SidecarInternalGRPCPort),
		"--dapr-listen-addresses", p.SidecarListenAddresses,
		"--dapr-public-port", strconv.Itoa(SidecarPublicPort),
		"--app-port", appPort,
		"--app-id", p.AppID,
		"--control-plane-address", p.ControlPlaneAddress,
		"--app-protocol", p.AppProtocol,
		"--placement-host-address", placementSvcAddress,
		"--config", p.Config,
		"--log-level", p.LogLevel,
		"--app-max-concurrency", strconv.FormatInt(int64(p.AppMaxConcurrency), 10),
		"--sentry-address", p.SentryAddress,
		"--enable-metrics=" + strconv.FormatBool(p.EnableMetrics),
		"--metrics-port", strconv.FormatInt(int64(p.MetricsPort), 10),
		"--dapr-http-max-request-size", strconv.FormatInt(int64(p.HTTPMaxRequestSize), 10),
		"--dapr-http-read-buffer-size", strconv.FormatInt(int64(p.HTTPReadBufferSize), 10),
		"--dapr-graceful-shutdown-seconds", strconv.FormatInt(int64(p.GracefulShutdownSeconds), 10),
		"--disable-builtin-k8s-secret-store=" + strconv.FormatBool(p.DisableBuiltinK8sSecretStore),
	}

	if p.AppChannelAddress != "" {
		args = append(args, "--app-channel-address", p.AppChannelAddress)
	}

	// --enable-api-logging is set only if it's non-nil (normally, if there's a specific annotation for that)
	// This is because if this CLI flag is missing, the default specified in the Config CRD is used
	if p.EnableAPILogging != nil {
		if *p.EnableAPILogging {
			args = append(args, "--enable-api-logging=true")
		} else {
			args = append(args, "--enable-api-logging=false")
		}
	}

	if p.EnableAppHealthCheck {
		args = append(args,
			"--enable-app-health-check=true",
			"--app-health-check-path", p.AppHealthCheckPath,
			"--app-health-probe-interval", strconv.FormatInt(int64(p.AppHealthProbeInterval), 10),
			"--app-health-probe-timeout", strconv.FormatInt(int64(p.AppHealthProbeTimeout), 10),
			"--app-health-threshold", strconv.FormatInt(int64(p.AppHealthThreshold), 10),
		)
	}

	if p.EnableDebug {
		ports = append(ports, corev1.ContainerPort{
			Name:          SidecarDebugPortName,
			ContainerPort: p.DebugPort,
		})

		cmd = []string{"/dlv"}

		args = append([]string{
			fmt.Sprintf("--listen=:%d", p.DebugPort),
			"--accept-multiclient",
			"--headless=true",
			"--log",
			"--api-version=2",
			"exec",
			"/daprd",
			"--",
		}, args...)
	}

	if image := cfg.Annotations.GetString(annotations.KeySidecarImage); image != "" {
		cfg.DaprSidecarImage = image
	}

	securityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.Of(false),
		RunAsNonRoot:             ptr.Of(p.RunAsNonRoot),
		ReadOnlyRootFilesystem:   ptr.Of(p.ReadOnlyRootFilesystem),
	}

	if p.SidecarSeccompProfileTyle != "" {
		securityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileType(p.SidecarSeccompProfileTyle),
		}
	}

	if p.SidecarDropALLCapabilities {
		securityContext.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	}

	container := &corev1.Container{
		Name:            SidecarContainerName,
		Image:           p.SidecarImage,
		ImagePullPolicy: p.ImagePullPolicy,
		SecurityContext: securityContext,
		Ports:           ports,
		Args:            append(cmd, args...),
		Env: []corev1.EnvVar{
			{
				Name:  "NAMESPACE",
				Value: p.Namespace,
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler:        probeHTTPHandler,
			InitialDelaySeconds: p.SidecarReadinessProbeDelaySeconds,
			TimeoutSeconds:      p.SidecarReadinessProbeTimeoutSeconds,
			PeriodSeconds:       p.SidecarReadinessProbePeriodSeconds,
			FailureThreshold:    p.SidecarReadinessProbeThreshold,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler:        probeHTTPHandler,
			InitialDelaySeconds: p.SidecarLivenessProbeDelaySeconds,
			TimeoutSeconds:      p.SidecarLivenessProbeTimeoutSeconds,
			PeriodSeconds:       p.SidecarLivenessProbePeriodSeconds,
			FailureThreshold:    p.SidecarLivenessProbeThreshold,
		},
	}

	// If the pod contains any of the tolerations specified by the configuration,
	// the Command and Args are passed as is. Otherwise, the Command is passed as a part of Args.
	// This is to allow the Docker images to specify an ENTRYPOINT
	// which is otherwise overridden by Command.
	if podContainsTolerations(p.IgnoreEntrypointTolerations, p.Tolerations) {
		container.Command = cmd
		container.Args = args
	} else {
		container.Args = cmd
		container.Args = append(container.Args, args...)
	}

	containerEnv := ParseEnvString(cfg.Annotations[annotations.KeyEnv])
	if len(containerEnv) > 0 {
		container.Env = append(container.Env, containerEnv...)
	}

	// This is a special case that requires administrator privileges in Windows containers
	// to install the certificates to the root store. If this environment variable is set,
	// the container security context should be set to run as administrator.
	for _, env := range container.Env {
		if env.Name == "SSL_CERT_DIR" {
			container.SecurityContext.WindowsOptions = &corev1.WindowsSecurityContextOptions{
				RunAsUserName: ptr.Of("ContainerAdministrator"),
			}

			// We also need to set RunAsNonRoot and ReadOnlyRootFilesystem to false, which would impact Linux too.
			// The injector has no way to know if the pod is going to be deployed on Windows or Linux, so we need to err on the side of most compatibility.
			// On Linux, our containers run with a non-root user, so the net effect shouldn't change: daprd is running as non-root and has no permission to write on the root FS.
			// However certain security scanner may complain about this.
			container.SecurityContext.RunAsNonRoot = ptr.Of(false)
			container.SecurityContext.ReadOnlyRootFilesystem = ptr.Of(false)
			break
		}
	}

	if len(cfg.VolumeMounts) > 0 {
		container.VolumeMounts = append(container.VolumeMounts, cfg.VolumeMounts...)
	}

	if cfg.ComponentsSocketsVolumeMount != nil {
		container.VolumeMounts = append(container.VolumeMounts, *cfg.ComponentsSocketsVolumeMount)
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  pluggable.SocketFolderEnvVar,
			Value: cfg.ComponentsSocketsVolumeMount.MountPath,
		})
	}

	if p.LogAsJSON {
		container.Args = append(container.Args, "--log-as-json")
	}

	if p.EnableProfiling {
		container.Args = append(container.Args, "--enable-profiling")
	}

	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  sentryConsts.TrustAnchorsEnvVar,
			Value: p.TrustAnchors,
		},
		corev1.EnvVar{
			Name:  sentryConsts.CertChainEnvVar,
			Value: p.CertChain,
		},
		corev1.EnvVar{
			Name:  sentryConsts.CertKeyEnvVar,
			Value: p.CertKey,
		},
		corev1.EnvVar{
			Name:  "SENTRY_LOCAL_IDENTITY",
			Value: p.Identity,
		},
	)

	if p.MTLSEnabled {
		container.Args = append(container.Args, "--enable-mtls")
	}

	if p.AppSSL {
		container.Args = append(container.Args, "--app-ssl")
	}

	if p.APITokenSecret != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: authConsts.APITokenEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "token",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.APITokenSecret,
					},
				},
			},
		})
	}

	if p.AppTokenSecret != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: authConsts.AppAPITokenEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "token",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: p.AppTokenSecret,
					},
				},
			},
		})
	}

	resources, err := p.getResourceRequirements()
	if err != nil {
		log.Warnf("couldn't set container resource requirements: %s. using defaults", err)
	}
	if resources != nil {
		container.Resources = *resources
	}
	return container, nil
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
		setValueFromString(fieldT.Type, fieldV, def)
	}
}

func (p *PodPatcher) getResourceRequirements() (*corev1.ResourceRequirements, error) {
	r := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}
	if p.SidecarCPURequest != "" {
		q, err := resource.ParseQuantity(p.SidecarCPURequest)
		if err != nil {
			return nil, fmt.Errorf("error parsing sidecar CPU request: %w", err)
		}
		r.Requests[corev1.ResourceCPU] = q
	}
	if p.SidecarCPULimit != "" {
		q, err := resource.ParseQuantity(p.SidecarCPULimit)
		if err != nil {
			return nil, fmt.Errorf("error parsing sidecar CPU limit: %w", err)
		}
		r.Limits[corev1.ResourceCPU] = q
	}
	if p.SidecarMemoryRequest != "" {
		q, err := resource.ParseQuantity(p.SidecarMemoryRequest)
		if err != nil {
			return nil, fmt.Errorf("error parsing sidecar memory request: %w", err)
		}
		r.Requests[corev1.ResourceMemory] = q
	}
	if p.SidecarMemoryLimit != "" {
		q, err := resource.ParseQuantity(p.SidecarMemoryLimit)
		if err != nil {
			return nil, fmt.Errorf("error parsing sidecar memory limit: %w", err)
		}
		r.Limits[corev1.ResourceMemory] = q
	}

	if len(r.Limits) == 0 && len(r.Requests) == 0 {
		return nil, nil
	}
	return &r, nil
}

func setValueFromString(rt reflect.Type, rv reflect.Value, val string) {
	switch rt.Kind() {
	case reflect.Pointer:
		pt := rt.Elem()
		pv := reflect.New(rt.Elem()).Elem()
		setValueFromString(pt, pv, val)
		rv.Set(pv.Addr())
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
