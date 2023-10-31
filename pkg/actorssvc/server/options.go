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

package server

import (
	"strings"
	"time"

	kclock "k8s.io/utils/clock"

	"github.com/dapr/components-contrib/actorstore"
	"github.com/dapr/components-contrib/metadata"
	loader "github.com/dapr/dapr/pkg/actorssvc/store"
	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
	"github.com/dapr/dapr/pkg/security"
)

type Options struct {
	Port int

	StoreName string
	StoreOpts []string

	HostHealthCheckInterval time.Duration

	EnableReminders              bool
	RemindersPollInterval        time.Duration
	RemindersFetchAheadInterval  time.Duration
	RemindersLeaseDuration       time.Duration
	RemindersFetchAheadBatchSize int

	Security security.Handler

	clock kclock.WithTicker
}

func (o Options) GetActorStore() (actorstore.Store, error) {
	return loader.DefaultRegistry.Create(o.StoreName)
}

func (o Options) GetActorStoreOptions() map[string]string {
	res := make(map[string]string, len(o.StoreOpts))
	for _, opt := range o.StoreOpts {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		k, v, _ := strings.Cut(opt, "=")
		res[k] = v
	}
	return res
}

func (o Options) GetActorStoreMetadata(pid string) actorstore.Metadata {
	return actorstore.Metadata{
		Base: metadata.Base{
			Properties: o.GetActorStoreOptions(),
		},
		PID:           pid,
		Configuration: o.GetActorsConfiguration(),
	}
}

func (o Options) GetActorsConfiguration() actorstore.ActorsConfiguration {
	return actorstore.ActorsConfiguration{
		HostHealthCheckInterval:      o.HostHealthCheckInterval,
		RemindersFetchAheadInterval:  o.RemindersFetchAheadInterval,
		RemindersLeaseDuration:       o.RemindersLeaseDuration,
		RemindersFetchAheadBatchSize: o.RemindersFetchAheadBatchSize,
	}
}

func (o Options) GetActorHostConfigurationMessage() *actorsv1pb.ConnectHostServerStream {
	return &actorsv1pb.ConnectHostServerStream{
		Message: &actorsv1pb.ConnectHostServerStream_ActorHostConfiguration{
			ActorHostConfiguration: &actorsv1pb.ActorHostConfiguration{
				HealthCheckInterval: uint32(o.HostHealthCheckInterval.Seconds()),
			},
		},
	}
}