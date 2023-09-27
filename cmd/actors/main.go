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

package main

import (
	"context"
	"fmt"

	// Register all stores
	_ "github.com/dapr/dapr/cmd/actors/stores"

	"github.com/dapr/dapr/cmd/actors/options"
	"github.com/dapr/dapr/pkg/actorssvc/monitoring"
	"github.com/dapr/dapr/pkg/actorssvc/server"
	loader "github.com/dapr/dapr/pkg/actorssvc/store"
	"github.com/dapr/dapr/pkg/buildinfo"
	"github.com/dapr/dapr/pkg/concurrency"
	"github.com/dapr/dapr/pkg/health"
	"github.com/dapr/dapr/pkg/metrics"
	"github.com/dapr/dapr/pkg/security"
	"github.com/dapr/dapr/pkg/signals"
	"github.com/dapr/kit/logger"
)

var (
	log        = logger.NewLogger("dapr.actorssvc")
	logContrib = logger.NewLogger("dapr.contrib")
)

func main() {
	opts := options.New()

	// Apply options to all loggers
	if err := logger.ApplyOptionsToLoggers(&opts.Logger); err != nil {
		log.Fatal(err)
	}

	log.Infof("Starting Dapr Actors service -- version %s -- commit %s", buildinfo.Version(), buildinfo.Commit())
	log.Infof("Log level set to: %s", opts.Logger.OutputLevel)

	loader.DefaultRegistry.Logger = logContrib

	metricsExporter := metrics.NewExporterWithOptions(log, metrics.DefaultMetricNamespace, opts.Metrics)
	err := monitoring.InitMetrics()
	if err != nil {
		log.Fatal(err)
	}

	ctx := signals.Context()

	// Security provider
	secProvider, err := security.New(ctx, security.Options{
		SentryAddress:           opts.SentryAddress,
		ControlPlaneTrustDomain: opts.TrustDomain,
		ControlPlaneNamespace:   security.CurrentNamespace(),
		TrustAnchorsFile:        opts.TrustAnchorsFile,
		AppID:                   "dapr-actors",
		MTLSEnabled:             opts.MTLSEnabled,
	})
	if err != nil {
		log.Fatal(err)
	}

	err = concurrency.NewRunnerManager(
		// Metrics exporter
		metricsExporter.Run,
		// Security provider
		secProvider.Run,
		// gRPC server
		func(ctx context.Context) error {
			sec, serr := secProvider.Handler(ctx)
			if serr != nil {
				return serr
			}
			return server.Start(ctx, server.Options{
				Port:                    opts.Port,
				StoreName:               opts.StoreName,
				StoreOpts:               opts.StoreOpts,
				HostHealthCheckInterval: opts.HostHealthCheckInterval,
				EnableReminders:         !opts.NoReminders,
				Security:                sec,
			})
		},
		// Healthz server
		func(ctx context.Context) error {
			healthzServer := health.NewServer(log)
			healthzServer.Ready()
			runErr := healthzServer.Run(ctx, opts.HealthzPort)
			if runErr != nil {
				return fmt.Errorf("failed to start healthz server: %w", runErr)
			}
			return nil
		},
	).Run(ctx)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Actors service shut down gracefully")
}
