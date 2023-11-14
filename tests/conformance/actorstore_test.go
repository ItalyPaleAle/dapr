//go:build conftests || e2e

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

package conformance

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	as_postgresql "github.com/dapr/dapr/pkg/actorssvc/components/actorstore/postgresql"
	conf_actorstore "github.com/dapr/dapr/tests/conformance/actorstore"
)

func TestActorStoreConformance(t *testing.T) {
	const configPath = "../config/actorstore/"
	tc, err := NewTestConfiguration(filepath.Join(configPath, "tests.yml"))
	require.NoError(t, err)
	require.NotNil(t, tc)

	tc.TestFn = func(comp *TestComponent) func(t *testing.T) {
		return func(t *testing.T) {
			ParseConfigurationMap(t, comp.Config)

			componentConfigPath := convertComponentNameToPath(comp.Component, comp.Profile)
			props, err := loadComponentsAndProperties(t, filepath.Join(configPath, componentConfigPath))
			require.NoErrorf(t, err, "error running conformance test for component %s", comp.Component)

			component := loadActorStore(comp.Component)
			require.NotNil(t, component, "error running conformance test for component %s", comp.Component)

			actorstoreConfig, err := conf_actorstore.NewTestConfig(comp.Component, comp.Operations, comp.Config)
			require.NoErrorf(t, err, "error running conformance test for component %s", comp.Component)

			conf_actorstore.ConformanceTests(t, props, component, actorstoreConfig)
		}
	}

	tc.Run(t)
}

func loadActorStore(name string) actorstore.Store {
	switch name {
	case "postgresql":
		return as_postgresql.NewPostgreSQLActorStore(testLogger)
	default:
		return nil
	}
}
