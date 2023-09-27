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
	"time"

	"github.com/dapr/components-contrib/actorstore"
)

// This file contains additional, hand-written methods added to the generated objects.

// Validate if the object contains the required fields.
func (x *ActorRef) Validate() error {
	if x.GetActorType() == "" {
		return errors.New("required property 'actor_type' is not set")
	}
	if x.GetActorId() == "" {
		return errors.New("required property 'actor_id' is not set")
	}
	return nil
}

// ToInternalActorRef converts the message to an actorstore.ActorRef object.
func (x *ActorRef) ToInternalActorRef() actorstore.ActorRef {
	return actorstore.ActorRef{
		ActorType: x.GetActorType(),
		ActorID:   x.GetActorId(),
	}
}

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

// GetActorTypeNames returns the list of all the names of supported actor types.
func (x *RegisterActorHost) GetActorTypeNames() []string {
	ats := x.GetActorTypes()
	res := make([]string, len(ats))
	n := 0
	for _, v := range ats {
		if v == nil {
			continue
		}
		res[n] = v.ActorType
		n++
	}
	return res[:n]
}

// ToActorStoreRequest converts the message to an actorstore.AddActorHostRequest object for registering a new actor host.
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
		APILevel:   x.ApiLevel,
		ActorTypes: actorTypes,
	}
}

// ToActorStoreUpdateRequest converts the message to an actorstore.UpdateActorHostRequest object for updating an actor host.
func (x *RegisterActorHost) ToUpdateActorHostRequest() actorstore.UpdateActorHostRequest {
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

	return actorstore.UpdateActorHostRequest{
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

// Validate that the message contains the required fields.
func (x *ActorHostConfiguration) Validate() error {
	if x == nil {
		return errors.New("object is nil")
	}
	if x.HealthCheckInterval <= 0 {
		return errors.New("required property 'health_check_interval' is not set")
	}
	return nil
}

// GetPingInterval returns the interval, as a time.Duration, to send pings according to the configuration.
func (x *ActorHostConfiguration) GetPingInterval() time.Duration {
	d := time.Duration(x.GetHealthCheckInterval()) * time.Second

	// We perform pings every d/2, but at most 5s before the expiration
	switch {
	case d > 10*time.Second:
		return d - 5*time.Second
	default:
		return d / 2
	}
}

// Validate if the message contains all fields that are required.
func (x *ReminderRef) Validate() error {
	if x == nil {
		return errors.New("message is nil")
	}
	if x.ActorType == "" {
		return errors.New("required property 'actor_type' is not set")
	}
	if x.ActorId == "" {
		return errors.New("required property 'actor_id' is not set")
	}
	if x.Name == "" {
		return errors.New("required property 'name' is not set")
	}
	return nil
}

// ToActorStoreRequest converts the message to an actorstore.ReminderRef object.
func (x *ReminderRef) ToActorStoreReminderRef() actorstore.ReminderRef {
	return actorstore.ReminderRef{
		ActorType: x.ActorType,
		ActorID:   x.ActorId,
		Name:      x.Name,
	}
}

// ValidateRequest validates if the message contains all fields that are required in the request.
func (x *Reminder) ValidateRequest() error {
	if x == nil {
		return errors.New("message is nil")
	}
	if x.ActorType == "" {
		return errors.New("required property 'actor_type' is not set")
	}
	if x.ActorId == "" {
		return errors.New("required property 'actor_id' is not set")
	}
	if x.Name == "" {
		return errors.New("required property 'name' is not set")
	}
	if x.ExecutionTime == nil || !x.ExecutionTime.IsValid() || x.ExecutionTime.AsTime().IsZero() {
		return errors.New("required property 'execution_time' is not set")
	}
	return nil
}

// ToActorStoreRequest converts the message to an actorstore.ReminderRef object.
func (x *Reminder) ToActorStoreReminderRef() actorstore.ReminderRef {
	return actorstore.ReminderRef{
		ActorType: x.ActorType,
		ActorID:   x.ActorId,
		Name:      x.Name,
	}
}

// ToActorStoreRequest converts the message to an actorstore.CreateReminderRequest object.
func (x *CreateReminderRequest) ToActorStoreRequest() actorstore.CreateReminderRequest {
	r := x.GetReminder()

	opts := actorstore.ReminderOptions{
		ExecutionTime: r.ExecutionTime.AsTime(),
	}
	if r.Period != "" {
		opts.Period = &r.Period
	}
	if r.Ttl != nil && r.Ttl.IsValid() {
		ttl := r.Ttl.AsTime()
		if !ttl.IsZero() {
			opts.TTL = &ttl
		}
	}
	if len(r.Data) > 0 {
		opts.Data = r.Data
	}

	return actorstore.CreateReminderRequest{
		ReminderRef:     r.ToActorStoreReminderRef(),
		ReminderOptions: opts,
	}
}

// ServerStreamMessage exposes isConnectHostServerStream_Message.
// It is the interface for messages that can be sent by the actor service to connected hosts.
type ServerStreamMessage = isConnectHostServerStream_Message

// ClientStreamMessage exposes isConnectHostClientStream_Message.
// It is the interface for messages that are sent from connected hosts to the server.
type ClientStreamMessage = isConnectHostClientStream_Message
