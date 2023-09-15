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

	"github.com/spf13/pflag"

	"github.com/dapr/dapr/pkg/actorssvc/config"
	"github.com/dapr/dapr/pkg/metrics"
	"github.com/dapr/dapr/pkg/security"
	securityConsts "github.com/dapr/dapr/pkg/security/consts"
	"github.com/dapr/kit/logger"
)

type Options struct {
	Port        int
	HealthzPort int

	DBName string
	DBOpts []string

	MTLSEnabled      bool
	TrustDomain      string
	TrustAnchorsFile string
	SentryAddress    string

	Logger  logger.Options
	Metrics *metrics.Options
}

func New() *Options {
	var opts Options

	fs := pflag.FlagSet{}

	fs.IntVar(&opts.Port, "port", config.DefaultPort, "The port for the sentry server to listen on")
	fs.IntVar(&opts.HealthzPort, "healthz-port", 8080, "The port for the healthz server to listen on")

	fs.StringVar(&opts.DBName, "db", "", "Name of the database driver")
	fs.StringArrayVar(&opts.DBOpts, "db-opt", nil, "Option for the database driver, in the format 'key=value'; can be repeated")

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

	if opts.DBName == "" {
		fmt.Println("Required flag '--db' was not provided")
		os.Exit(2)
	}

	return &opts
}
