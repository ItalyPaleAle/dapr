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

package sentry

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/dapr/dapr/pkg/actorssvc/config"
	"github.com/dapr/kit/logger"
)

var log = logger.NewLogger("dapr.actorssvc")

// ActorsService is the interface for the Actors service.
type ActorsService interface {
	// Start the gRPC server.
	Start(context.Context) error
}

type service struct {
	conf    config.Config
	running atomic.Bool
}

// New returns a new instance of the Actors service.
func New(conf config.Config) ActorsService {
	return &service{
		conf: conf,
	}
}

// Start the server in background.
func (s *service) Start(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// If the server is already running, return an error
	if !s.running.CompareAndSwap(false, true) {
		return errors.New("server is already running")
	}

	_ = ctx

	return nil
}
