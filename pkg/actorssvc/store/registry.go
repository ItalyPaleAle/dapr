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

package store

import (
	"fmt"
	"strings"

	"github.com/dapr/components-contrib/actorstore"
	"github.com/dapr/kit/logger"
)

// Registry is used to get registered actor store implementations.
type Registry struct {
	Logger logger.Logger
	stores map[string]FactoryFn
}

type FactoryFn = func(logger.Logger) actorstore.Store

// DefaultRegistry is the singleton with the registry.
var DefaultRegistry *Registry

func init() {
	DefaultRegistry = NewRegistry()
}

// NewRegistry returns a new actor store registry.
func NewRegistry() *Registry {
	return &Registry{
		stores: map[string]FactoryFn{},
	}
}

// RegisterStore adds a new actor store to the registry.
func (s *Registry) RegisterStore(factory FactoryFn, names ...string) {
	for _, name := range names {
		s.stores[name] = factory
	}
}

// Create instantiates a actor store based on `name`.
func (s *Registry) Create(name string) (actorstore.Store, error) {
	factory, ok := s.getWrappedFactory(name)
	if !ok {
		return nil, fmt.Errorf("couldn't find actor store %s", name)
	}

	return factory(), nil
}

func (s *Registry) getWrappedFactory(name string) (func() actorstore.Store, bool) {
	name = strings.ToLower(name)
	factory, ok := s.stores[name]
	if ok {
		return s.wrapFn(factory, name), true
	}
	return nil, false
}

func (s *Registry) wrapFn(factory FactoryFn, logName string) func() actorstore.Store {
	return func() actorstore.Store {
		l := s.Logger
		if logName != "" && l != nil {
			l = l.WithFields(map[string]any{
				"store": logName,
			})
		}
		return factory(l)
	}
}
