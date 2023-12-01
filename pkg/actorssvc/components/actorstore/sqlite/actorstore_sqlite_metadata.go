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

package sqlite

import (
	"time"

	authSqlite "github.com/dapr/components-contrib/common/authentication/sqlite"
	"github.com/dapr/dapr/pkg/actorssvc/components/actorstore"
	"github.com/dapr/kit/metadata"
)

type sqliteMetadata struct {
	authSqlite.SqliteAuthMetadata `mapstructure:",squash"`

	PID    string                         `mapstructure:"-"`
	Config actorstore.ActorsConfiguration `mapstructure:"-"`
}

func (m *sqliteMetadata) InitWithMetadata(meta actorstore.Metadata) error {
	// Reset the object
	m.SqliteAuthMetadata.Reset()
	m.Timeout = 20 * time.Second

	// Copy the PID and configuration
	m.PID = meta.PID
	m.Config = meta.Configuration

	// Decode the metadata
	err := metadata.DecodeMetadata(meta.Properties, &m)
	if err != nil {
		return err
	}

	// Validate and sanitize input
	err = m.SqliteAuthMetadata.Validate()
	if err != nil {
		return err
	}

	return nil
}
