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
	"math"
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
	// DefaultHostHealthCheckInterval is the default interval between pings received from an actor host.
	DefaultHostHealthCheckInterval = 30 * time.Second
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

	HostHealthCheckInterval time.Duration

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

	fs.DurationVar(&opts.HostHealthCheckInterval, "host-healthcheck-interval", DefaultHostHealthCheckInterval, "Interval for expecting health checks from actor hosts")

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

	validationErr := opts.Validate()
	if validationErr != "" {
		fmt.Println(validationErr)
		os.Exit(2)
	}

	return &opts
}

func (o Options) Validate() string {
	if o.StoreName == "" {
		return "Required flag '--store' is missing"
	}
	if o.HostHealthCheckInterval < time.Second {
		return "Flag '--host-healthcheck-interval' must be at least '1s'"
	}
	if o.HostHealthCheckInterval != o.HostHealthCheckInterval.Truncate(time.Second) {
		return "Flag '--host-healthcheck-interval' must not have fractions of seconds"
	}
	if o.HostHealthCheckInterval.Seconds() > math.MaxUint32 {
		// HealthCheckInterval can be cast into an uint32 (in seconds), so we need to make sure we don't overflow
		return "Flag '--host-healthcheck-interval' is too big"
	}
	return ""
}
