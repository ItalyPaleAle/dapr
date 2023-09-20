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

	"github.com/dapr/components-contrib/actorstore"
	"github.com/dapr/components-contrib/metadata"
	loader "github.com/dapr/dapr/pkg/actorssvc/store"
	"github.com/dapr/dapr/pkg/security"
)

type Options struct {
	Port int

	StoreName string
	StoreOpts []string

	HostHealthCheckInterval time.Duration

	Security security.Handler
}

func (o Options) GetActorStore() (actorstore.Store, error) {
	return loader.DefaultRegistry.Create(o.StoreName)
}

func (o Options) GetActorStoreOptions() map[string]string {
	res := make(map[string]string, len(o.StoreOpts))
	for _, opt := range o.StoreOpts {
		k, v, _ := strings.Cut(opt, "=")
		res[k] = v
	}
	return res
}

func (o Options) GetActorStoreMetadata() actorstore.Metadata {
	return actorstore.Metadata{
		Base: metadata.Base{
			Properties: o.GetActorStoreOptions(),
		},
		Configuration: actorstore.ActorsConfiguration{
			HostHealthCheckInterval: o.HostHealthCheckInterval,
		},
	}
}
