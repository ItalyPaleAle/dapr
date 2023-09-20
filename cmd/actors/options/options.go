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

package options

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"

	"github.com/dapr/dapr/pkg/metrics"
	"github.com/dapr/dapr/pkg/security"
	securityConsts "github.com/dapr/dapr/pkg/security/consts"
	"github.com/dapr/kit/logger"
)

const (
	// DefaultPort is the default port for the actors service to listen on.
	DefaultPort = 51101
	// DefaultHealthzPort is the default port for the healthz server to listen on.
	DefaultHealthzPort = 8080
	// DefaultHostHealthCheckInterval is the default interval for performing health-checks on actor hosts.
	DefaultHostHealthCheckInterval = 10 * time.Second
	// DefaultHostHealthCheckTimeout is the default timeout for performing health-checks on actor hosts.
	DefaultHostHealthCheckTimeout = 2 * time.Second
	// DefaultHostHealthCheckThreshold is the default number of failed health-checks after which an actor host is considered unhealthy.
	DefaultHostHealthCheckThreshold = 2
)

type Options struct {
	Port        int
	HealthzPort int

	StoreName string
	StoreOpts []string

	MTLSEnabled      bool
	TrustDomain      string
	TrustAnchorsFile string
	SentryAddress    string

	HostHealthCheckInterval  time.Duration
	HostHealthCheckTimeout   time.Duration
	HostHealthCheckThreshold int

	Logger  logger.Options
	Metrics *metrics.Options
}

func New() *Options {
	var opts Options

	fs := pflag.FlagSet{}

	fs.IntVar(&opts.Port, "port", DefaultPort, "The port for the sentry server to listen on")
	fs.IntVar(&opts.HealthzPort, "healthz-port", DefaultHealthzPort, "The port for the healthz server to listen on")

	fs.StringVar(&opts.StoreName, "store", "", "Name of the store driver")
	fs.StringArrayVar(&opts.StoreOpts, "store-opt", nil, "Option for the store driver, in the format 'key=value'; can be repeated")

	fs.DurationVar(&opts.HostHealthCheckInterval, "host-healthcheck-interval", DefaultHostHealthCheckInterval, "Interval for performing health checks on actor hosts, as a duration")
	fs.DurationVar(&opts.HostHealthCheckTimeout, "host-healthcheck-timeout", DefaultHostHealthCheckTimeout, "Timeout for actor host health checks, as a duration")
	fs.IntVar(&opts.HostHealthCheckThreshold, "host-healthcheck-threshold", DefaultHostHealthCheckThreshold, "Number of consecutive failures for an actor host to be considered unhealthy")

	fs.BoolVar(&opts.MTLSEnabled, "enable-mtls", false, "Enable mTLS")
	fs.StringVar(&opts.TrustDomain, "trust-domain", "localhost", "Trust domain for the Dapr control plane (for mTLS)")
	fs.StringVar(&opts.TrustAnchorsFile, "trust-anchors-file", securityConsts.ControlPlaneDefaultTrustAnchorsPath, "Filepath to the trust anchors for the Dapr control plane (for mTLS)")
	fs.StringVar(&opts.SentryAddress, "sentry-address", fmt.Sprintf("dapr-sentry.%s.svc:443", security.CurrentNamespace()), "Address of the Sentry service (for mTLS)")

	opts.Logger = logger.DefaultOptions()
	opts.Logger.AttachCmdFlags(fs.StringVar, fs.BoolVar)

	opts.Metrics = metrics.DefaultMetricOptions()
	opts.Metrics.AttachCmdFlags(fs.StringVar, fs.BoolVar)

	fs.SortFlags = false
	fs.Parse(os.Args[1:])

	if opts.StoreName == "" {
		fmt.Println("Required flag '--store' is missing")
		os.Exit(2)
	}
	if opts.HostHealthCheckInterval < time.Second {
		fmt.Println("Flag '--host-healthcheck-interval' must be at least '1s'")
		os.Exit(2)
	}
	if opts.HostHealthCheckTimeout < 50*time.Millisecond {
		fmt.Println("Flag '--host-healthcheck-timeout' must be at least '50ms'")
		os.Exit(2)
	}
	if time.Duration(opts.HostHealthCheckThreshold) < 1 {
		fmt.Println("Flag '--host-healthcheck-threshold' must be at least '1'")
		os.Exit(2)
	}

	return &opts
}
