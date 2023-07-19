package injector

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/dapr/dapr/pkg/components/pluggable"
	authConsts "github.com/dapr/dapr/pkg/runtime/security/consts"
	securityConsts "github.com/dapr/dapr/pkg/sentry/consts"
	"github.com/dapr/dapr/utils"
	"github.com/dapr/kit/ptr"
)

type getSidecarContainerOpts struct {
	VolumeMounts                 []corev1.VolumeMount
	ComponentsSocketsVolumeMount *corev1.VolumeMount
}

// GetSidecarContainer returns the Container object for the sidecar.
func (cfg SidecarConfig) GetSidecarContainer(opts getSidecarContainerOpts) (*corev1.Container, error) {
	// TODO: In caller, set defaults for PlacementServiceAddress and SidecarImage
	// We still include PlacementServiceAddress if explicitly set as annotation
	/*if cfg.Annotations.Exist(annotations.KeyPlacementHostAddresses) {
		cfg.PlacementServiceAddress = cfg.Annotations.GetString(annotations.KeyPlacementHostAddresses)
	} else if cfg.SkipPlacement {
		cfg.PlacementServiceAddress = ""
	}
	if image := cfg.Annotations.GetString(annotations.KeySidecarImage); image != "" {
		cfg.DaprSidecarImage = image
	}*/
	// TODO: If UDS are used, add the volume
	/*
		// socketVolume is an EmptyDir
		socketVolume := &corev1.Volume{
			Name: UnixDomainSocketVolume,
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, *socketVolume)
	*/

	// Ports for the daprd container
	ports := []corev1.ContainerPort{
		{
			ContainerPort: cfg.SidecarHTTPPort,
			Name:          SidecarHTTPPortName,
		},
		{
			ContainerPort: cfg.SidecarAPIGRPCPort,
			Name:          SidecarGRPCPortName,
		},
		{
			ContainerPort: cfg.SidecarInternalGRPCPort,
			Name:          SidecarInternalGRPCPortName,
		},
		{
			ContainerPort: cfg.SidecarMetricsPort,
			Name:          SidecarMetricsPortName,
		},
	}

	// Get the command (/daprd) and all CLI flags
	cmd := []string{"/daprd"}
	args := []string{
		"--dapr-http-port", strconv.FormatInt(int64(cfg.SidecarHTTPPort), 10),
		"--dapr-grpc-port", strconv.FormatInt(int64(cfg.SidecarAPIGRPCPort), 10),
		"--dapr-internal-grpc-port", strconv.FormatInt(int64(cfg.SidecarInternalGRPCPort), 10),
		"--dapr-listen-addresses", cfg.SidecarListenAddresses,
		"--dapr-public-port", strconv.FormatInt(int64(cfg.SidecarPublicPort), 10),
		"--app-id", cfg.GetAppID(),
		"--app-protocol", cfg.AppProtocol,
		"--control-plane-address", cfg.OperatorAddress,
		"--sentry-address", cfg.SentryAddress,
		"--log-level", cfg.LogLevel,
		"--app-max-concurrency", strconv.Itoa(cfg.AppMaxConcurrency),
		"--dapr-http-max-request-size", strconv.Itoa(cfg.HTTPMaxRequestSize),
		"--dapr-http-read-buffer-size", strconv.Itoa(cfg.HTTPReadBufferSize),
		"--dapr-graceful-shutdown-seconds", strconv.Itoa(cfg.GracefulShutdownSeconds),
	}

	if cfg.KubernetesMode {
		args = append(args, "--mode", "kubernetes")
	}

	if cfg.AppPort > 0 {
		args = append(args, "--app-port", strconv.FormatInt(int64(cfg.AppPort), 10))
	}

	if cfg.EnableMetrics {
		args = append(args,
			"--enable-metrics",
			"--metrics-port", strconv.FormatInt(int64(cfg.SidecarMetricsPort), 10),
		)
	}

	if cfg.Config != "" {
		args = append(args, "--config", cfg.Config)
	}

	if cfg.AppChannelAddress != "" {
		args = append(args, "--app-channel-address", cfg.AppChannelAddress)
	}

	// Placement address could be empty if placement service is disabled
	if cfg.PlacementAddress != "" {
		args = append(args, "--placement-host-address", cfg.PlacementAddress)
	}

	// --enable-api-logging is set if and only if there's an explicit value (true or false) for that
	// This is set explicitly even if "false"
	// This is because if this CLI flag is missing, the default specified in the Config CRD is used
	if cfg.EnableAPILogging != nil {
		args = append(args, "--enable-api-logging", strconv.FormatBool(*cfg.EnableAPILogging))
	}

	if cfg.DisableBuiltinK8sSecretStore {
		args = append(args, "--disable-builtin-k8s-secret-store")
	}

	if cfg.EnableAppHealthCheck {
		args = append(args,
			"--enable-app-health-check",
			"--app-health-check-path", cfg.AppHealthCheckPath,
			"--app-health-probe-interval", strconv.FormatInt(int64(cfg.AppHealthProbeInterval), 10),
			"--app-health-probe-timeout", strconv.FormatInt(int64(cfg.AppHealthProbeTimeout), 10),
			"--app-health-threshold", strconv.FormatInt(int64(cfg.AppHealthThreshold), 10),
		)
	}

	if cfg.LogAsJSON {
		args = append(args, "--log-as-json")
	}

	if cfg.EnableProfiling {
		args = append(args, "--enable-profiling")
	}

	if cfg.MTLSEnabled {
		args = append(args, "--enable-mtls")
	}

	if cfg.AppSSL {
		args = append(args, "--app-ssl")
	}

	// When debugging is enabled, we need to override the command and the flags
	if cfg.EnableDebug {
		ports = append(ports, corev1.ContainerPort{
			Name:          SidecarDebugPortName,
			ContainerPort: cfg.SidecarDebugPort,
		})

		cmd = []string{"/dlv"}

		args = append([]string{
			"--listen", strconv.FormatInt(int64(cfg.SidecarDebugPort), 10),
			"--accept-multiclient",
			"--headless=true",
			"--log",
			"--api-version=2",
			"exec",
			"/daprd",
			"--",
		}, args...)
	}

	// Security context
	securityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.Of(false),
		RunAsNonRoot:             ptr.Of(cfg.RunAsNonRoot),
		ReadOnlyRootFilesystem:   ptr.Of(cfg.ReadOnlyRootFilesystem),
	}
	if cfg.SidecarSeccompProfileType != "" {
		securityContext.SeccompProfile = &corev1.SeccompProfile{
			Type: corev1.SeccompProfileType(cfg.SidecarSeccompProfileType),
		}
	}
	if cfg.SidecarDropALLCapabilities {
		securityContext.Capabilities = &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		}
	}

	// Get all volume mounts
	volumeMounts := cfg.GetVolumeMounts()
	if socketVolumeMount := cfg.GetUnixDomainSocketVolumeMount(); socketVolumeMount != nil {
		volumeMounts = append(volumeMounts, *socketVolumeMount)
	}
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      TokenVolumeName,
		MountPath: TokenVolumeKubernetesMountPath,
		ReadOnly:  true,
	})

	// Create the container object
	probeHTTPHandler := getProbeHTTPHandler(cfg.SidecarPublicPort, APIVersionV1, SidecarHealthzPath)
	container := &corev1.Container{
		Name:            SidecarContainerName,
		Image:           cfg.SidecarImage,
		ImagePullPolicy: cfg.ImagePullPolicy,
		SecurityContext: securityContext,
		Ports:           ports,
		Args:            append(cmd, args...),
		Env: []corev1.EnvVar{
			{
				Name:  "NAMESPACE",
				Value: cfg.Namespace,
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
		VolumeMounts: volumeMounts,
		ReadinessProbe: &corev1.Probe{
			ProbeHandler:        probeHTTPHandler,
			InitialDelaySeconds: cfg.SidecarReadinessProbeDelaySeconds,
			TimeoutSeconds:      cfg.SidecarReadinessProbeTimeoutSeconds,
			PeriodSeconds:       cfg.SidecarReadinessProbePeriodSeconds,
			FailureThreshold:    cfg.SidecarReadinessProbeThreshold,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler:        probeHTTPHandler,
			InitialDelaySeconds: cfg.SidecarLivenessProbeDelaySeconds,
			TimeoutSeconds:      cfg.SidecarLivenessProbeTimeoutSeconds,
			PeriodSeconds:       cfg.SidecarLivenessProbePeriodSeconds,
			FailureThreshold:    cfg.SidecarLivenessProbeThreshold,
		},
	}

	// If the pod contains any of the tolerations specified by the configuration,
	// the Command and Args are passed as is. Otherwise, the Command is passed as a part of Args.
	// This is to allow the Docker images to specify an ENTRYPOINT
	// which is otherwise overridden by Command.
	if podContainsTolerations(cfg.IgnoreEntrypointTolerations, cfg.Tolerations) {
		container.Command = cmd
		container.Args = args
	} else {
		container.Args = cmd
		container.Args = append(container.Args, args...)
	}

	// Set env vars if needed
	containerEnvKeys, containerEnv := cfg.GetEnv()
	if len(containerEnv) > 0 {
		container.Env = append(container.Env, containerEnv...)
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  authConsts.EnvKeysEnvVar,
			Value: strings.Join(containerEnvKeys, " "),
		})
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

	if opts.ComponentsSocketsVolumeMount != nil {
		container.VolumeMounts = append(container.VolumeMounts, *opts.ComponentsSocketsVolumeMount)
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  pluggable.SocketFolderEnvVar,
			Value: opts.ComponentsSocketsVolumeMount.MountPath,
		})
	}

	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  securityConsts.TrustAnchorsEnvVar,
			Value: cfg.TrustAnchors,
		},
		corev1.EnvVar{
			Name:  securityConsts.CertChainEnvVar,
			Value: cfg.CertChain,
		},
		corev1.EnvVar{
			Name:  securityConsts.CertKeyEnvVar,
			Value: cfg.CertKey,
		},
		corev1.EnvVar{
			Name:  "SENTRY_LOCAL_IDENTITY",
			Value: cfg.Identity,
		},
	)

	if cfg.APITokenSecret != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: authConsts.APITokenEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "token",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.APITokenSecret,
					},
				},
			},
		})
	}

	if cfg.AppTokenSecret != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name: authConsts.AppAPITokenEnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "token",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.AppTokenSecret,
					},
				},
			},
		})
	}

	// Resources for the container
	resources, err := cfg.getResourceRequirements()
	if err != nil {
		log.Warnf("couldn't set container resource requirements: %s. using defaults", err)
	} else if resources != nil {
		container.Resources = *resources
	}

	return container, nil
}

func (c *SidecarConfig) getResourceRequirements() (*corev1.ResourceRequirements, error) {
	r := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}
	if c.SidecarCPURequest != "" {
		q, err := resource.ParseQuantity(c.SidecarCPURequest)
		if err != nil {
			return nil, fmt.Errorf("error parsing sidecar CPU request: %w", err)
		}
		r.Requests[corev1.ResourceCPU] = q
	}
	if c.SidecarCPULimit != "" {
		q, err := resource.ParseQuantity(c.SidecarCPULimit)
		if err != nil {
			return nil, fmt.Errorf("error parsing sidecar CPU limit: %w", err)
		}
		r.Limits[corev1.ResourceCPU] = q
	}
	if c.SidecarMemoryRequest != "" {
		q, err := resource.ParseQuantity(c.SidecarMemoryRequest)
		if err != nil {
			return nil, fmt.Errorf("error parsing sidecar memory request: %w", err)
		}
		r.Requests[corev1.ResourceMemory] = q
	}
	if c.SidecarMemoryLimit != "" {
		q, err := resource.ParseQuantity(c.SidecarMemoryLimit)
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

// GetAppID returns the AppID property, fallinb back to the name of the pod.
func (c *SidecarConfig) GetAppID() string {
	if c.AppID == "" {
		return c.pod.GetName()
	}

	return c.AppID
}

var envRegexp = regexp.MustCompile(`(?m)(,)\s*[a-zA-Z\_][a-zA-Z0-9\_]*=`)

// GetEnv returns the EnvVar slice from the Env annotation.
func (c *SidecarConfig) GetEnv() (envKeys []string, envVars []corev1.EnvVar) {
	if c.Env == "" {
		return nil, nil
	}

	indexes := envRegexp.FindAllStringIndex(c.Env, -1)
	lastEnd := len(c.Env)
	parts := make([]string, len(indexes)+1)
	for i := len(indexes) - 1; i >= 0; i-- {
		parts[i+1] = strings.TrimSpace(c.Env[indexes[i][0]+1 : lastEnd])
		lastEnd = indexes[i][0]
	}
	parts[0] = c.Env[0:lastEnd]

	envKeys = make([]string, 0, len(parts))
	envVars = make([]corev1.EnvVar, 0, len(parts))
	for _, s := range parts {
		pairs := strings.Split(strings.TrimSpace(s), "=")
		if len(pairs) != 2 {
			continue
		}
		envKeys = append(envKeys, pairs[0])
		envVars = append(envVars, corev1.EnvVar{
			Name:  pairs[0],
			Value: pairs[1],
		})
	}

	return envKeys, envVars
}

func getProbeHTTPHandler(port int32, pathElements ...string) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: formatProbePath(pathElements...),
			Port: intstr.IntOrString{IntVal: port},
		},
	}
}

func formatProbePath(elements ...string) string {
	pathStr := path.Join(elements...)
	if !strings.HasPrefix(pathStr, "/") {
		pathStr = "/" + pathStr
	}
	return pathStr
}

// podContainsTolerations returns true if the pod contains any of the tolerations specified in ts.
func podContainsTolerations(ts []corev1.Toleration, podTolerations []corev1.Toleration) bool {
	if len(ts) == 0 || len(podTolerations) == 0 {
		return false
	}

	// If the pod contains any of the tolerations specified, return true.
	for _, t := range ts {
		if utils.Contains(podTolerations, t) {
			return true
		}
	}

	return false
}
