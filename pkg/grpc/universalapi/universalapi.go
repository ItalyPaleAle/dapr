/*
Copyright 2022 The Dapr Authors
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

// Package universalapi contains the implementation of APIs that are shared between gRPC and HTTP servers.
// On HTTP servers, they use protojson to convert data to/from JSON.
package universalapi

import (
	"github.com/dapr/components-contrib/secretstores"
	"github.com/dapr/dapr/pkg/actorsv2"
	"github.com/dapr/dapr/pkg/config"
	"github.com/dapr/dapr/pkg/resiliency"
	"github.com/dapr/kit/logger"
)

// UniversalAPI contains the implementation of gRPC APIs that are also used by the HTTP server.
type UniversalAPI struct {
	Logger               logger.Logger
	Resiliency           resiliency.Provider
	ActorV2              actorsv2.Actors
	SecretStores         map[string]secretstores.SecretStore
	SecretsConfiguration map[string]config.SecretsScope
}
