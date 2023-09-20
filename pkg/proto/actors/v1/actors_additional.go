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

package actors

import (
	"errors"

	"github.com/dapr/components-contrib/actorstore"
)

// This file contains additional, hand-written methods added to the generated objects.

// ValidateFirstMessage validates if the message contains all fields that are required in the first message.
func (x *RegisterActorHost) ValidateFirstMessage() error {
	const errPrefix = "first message validation failed: "
	if x == nil {
		return errors.New(errPrefix + "message is nil")
	}
	if x.Address == "" {
		return errors.New(errPrefix + "required property 'address' is not set")
	}
	if x.AppId == "" {
		return errors.New(errPrefix + "required property 'app_id' is not set")
	}
	if x.ApiLevel <= 0 {
		return errors.New(errPrefix + "required property 'api_level' is not set")
	}
	return nil
}

// ValidateUpdateMessage validates a message sent to update the registration.
func (x *RegisterActorHost) ValidateUpdateMessage() error {
	const errPrefix = "update message validation failed: "
	if x.GetAddress() != "" {
		return errors.New(errPrefix + "property 'address' cannot be updated")
	}
	if x.GetAppId() != "" {
		return errors.New(errPrefix + "property 'app_id' cannot be updated")
	}
	if x.GetApiLevel() > 0 {
		return errors.New(errPrefix + "property 'api_level' cannot be updated")
	}
	return nil
}

// ToActorStoreRequest converts the message to an actorstore.AddActorHostRequest object.
func (x *RegisterActorHost) ToActorStoreRequest() actorstore.AddActorHostRequest {
	var actorTypes []actorstore.ActorHostType = nil
	if x.ActorTypes != nil {
		var n int
		actorTypes = make([]actorstore.ActorHostType, len(x.ActorTypes))
		for _, at := range x.ActorTypes {
			if at == nil {
				continue
			}
			actorTypes[n] = at.ToActorStoreRequest()
			n++
		}
		actorTypes = actorTypes[:n]
	}

	return actorstore.AddActorHostRequest{
		AppID:      x.AppId,
		Address:    x.Address,
		ApiLevel:   x.ApiLevel,
		ActorTypes: actorTypes,
	}
}

// ToActorStoreRequest converts the message to an actorstore.ActorHostType object.
func (x *ActorHostType) ToActorStoreRequest() actorstore.ActorHostType {
	return actorstore.ActorHostType{
		ActorType:   x.ActorType,
		IdleTimeout: x.IdleTimeout,
	}
}
