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

package internal

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	actorsv1pb "github.com/dapr/dapr/pkg/proto/actors/v1"
)

// ToProto returns the Reminder protobuf object.
func (r Reminder) ToProto() *actorsv1pb.Reminder {
	p := &actorsv1pb.Reminder{
		ActorType:     r.ActorType,
		ActorId:       r.ActorID,
		Name:          r.Name,
		ExecutionTime: timestamppb.New(r.RegisteredTime),
		Period:        r.Period.String(),
	}

	if !r.ExpirationTime.IsZero() {
		p.Ttl = timestamppb.New(r.ExpirationTime)
	}

	if len(r.Data) > 0 {
		p.Data = r.Data
	}

	return p
}

// NewReminderFromProto returns a Reminder object from the protobuf object.
func NewReminderFromProto(p *actorsv1pb.Reminder) *Reminder {
	r := &Reminder{
		ActorType:      p.ActorType,
		ActorID:        p.ActorId,
		Name:           p.Name,
		RegisteredTime: p.ExecutionTime.AsTime(),
	}

	if p.Period != "" {
		// We can ignore errors here because we know the period is valid
		r.Period, _ = NewReminderPeriod(p.Period)
	}

	if p.Ttl != nil && p.Ttl.IsValid() {
		r.ExpirationTime = p.Ttl.AsTime()
	}

	if len(p.Data) > 0 {
		r.Data = p.Data
	}

	return r
}

// ToRefProto gets the actorsv1pb.ReminderRef object for this request.
func (req GetReminderRequest) ToRefProto() *actorsv1pb.ReminderRef {
	return &actorsv1pb.ReminderRef{
		Name:      req.Name,
		ActorType: req.ActorType,
		ActorId:   req.ActorID,
	}
}

// ToRefProto gets the actorsv1pb.ReminderRef object for this request.
func (req DeleteReminderRequest) ToRefProto() *actorsv1pb.ReminderRef {
	return &actorsv1pb.ReminderRef{
		Name:      req.Name,
		ActorType: req.ActorType,
		ActorId:   req.ActorID,
	}
}
