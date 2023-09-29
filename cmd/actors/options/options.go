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
	"bufio"
	"errors"
	"fmt"
	"io"
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
	// DefaultRemindersPollInterval is the default polling interval for reminders.
	// Clusters with lots of high-frequency reminders may want to set this to a lower value.
	DefaultRemindersPollInterval = 2500 * time.Millisecond
	// DefaultRemindersFetchAheadInterval is the default duration for pre-fetching reminders.
	// The fetch ahead interval must be greater than the poll interval.
	DefaultRemindersFetchAheadInterval = 5 * time.Second
	// DefaultRemindersLeaseDuration is the default duration for leases in the reminders table.
	DefaultRemindersLeaseDuration = 20 * time.Second
	// DefaultRemindersFetchAheadBatchSize is the default batch size for pre-fetching reminders.
	DefaultRemindersFetchAheadBatchSize = 50
)

type Options struct {
	Port        int
	HealthzPort int

	StoreName     string
	StoreOpts     []string
	StoreOptsFile string

	MTLSEnabled      bool
	TrustDomain      string
	TrustAnchorsFile string
	SentryAddress    string

	HostHealthCheckInterval time.Duration

	NoReminders                  bool
	RemindersPollInterval        time.Duration
	RemindersFetchAheadInterval  time.Duration
	RemindersLeaseDuration       time.Duration
	RemindersFetchAheadBatchSize int

	Logger  logger.Options
	Metrics *metrics.Options
}

func New() *Options {
	var opts Options

	fs := pflag.FlagSet{}

	fs.IntVar(&opts.Port, "port", DefaultPort, "The port for the sentry server to listen on")
	fs.IntVar(&opts.HealthzPort, "healthz-port", DefaultHealthzPort, "The port for the healthz server to listen on")

	fs.StringVar(&opts.StoreName, "store-name", "", "Name of the store driver")
	fs.StringArrayVar(&opts.StoreOpts, "store-opt", nil, "Option for the store driver, in the format 'key=value'; can be repeated")
	fs.StringVar(&opts.StoreOptsFile, "store-opts-file", "", "Path to a file containing options for the store, with each line having its own 'key=value'")

	fs.DurationVar(&opts.HostHealthCheckInterval, "host-healthcheck-interval", DefaultHostHealthCheckInterval, "Interval for expecting health checks from actor hosts")

	fs.BoolVar(&opts.NoReminders, "no-reminders", false, "If true, does not enable reminders functionality in the service")
	fs.DurationVar(&opts.RemindersPollInterval, "reminders-poll-interval", DefaultRemindersPollInterval, "Polling interval for fetching reminders from the store. Clusters with lots of high-frequency reminders may want to use smaller values, at the expense of increased load on the store")
	fs.DurationVar(&opts.RemindersFetchAheadInterval, "reminders-fetch-ahead-interval", DefaultRemindersFetchAheadInterval, "Pre-fetches reminders that are set to be executed up to this interval in the future. Must be greater than the polling interval.")
	fs.DurationVar(&opts.RemindersLeaseDuration, "reminders-lease-duration", DefaultRemindersLeaseDuration, "Duration for leases acquired in the reminders table")
	fs.IntVar(&opts.RemindersFetchAheadBatchSize, "reminders-fetch-ahead-batch-size", DefaultRemindersFetchAheadBatchSize, "Maximum number of reminders fetched in each batch. Clusters with lots of reminders should either scale the Actors service horizontally or increase this value.")

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

	err := opts.LoadStoreOptionsFile()
	if err != nil {
		//nolint:forbidigo
		fmt.Println("Failed to load options from file:", err)
		os.Exit(2)
	}

	validationErr := opts.Validate()
	if validationErr != "" {
		//nolint:forbidigo
		fmt.Println(validationErr)
		os.Exit(2)
	}

	return &opts
}

func (o Options) Validate() string {
	if o.StoreName == "" {
		return "Required flag '--store-name' is missing"
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

func (o *Options) LoadStoreOptionsFile() error {
	if o.StoreOptsFile == "" {
		return nil
	}

	f, err := os.Open(o.StoreOptsFile)
	if err != nil {
		return fmt.Errorf("failed to open file with store options: %w", err)
	}
	defer f.Close()

	bio := bufio.NewReader(f)
	for {
		line, isPrefix, err := bio.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read from file with store options: %w", err)
		}
		if isPrefix {
			return errors.New("failed to read from file with store options: file contains lines too long")
		}
		o.StoreOpts = append(o.StoreOpts, string(line))
	}

	return nil
}
