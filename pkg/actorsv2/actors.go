package actorsv2

import (
	"context"

	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
)

// Actors allow calling into virtual actors as well as actor state management.
type Actors interface {
	Init() error
	Close() error

	Call(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error)

	GetState(ctx context.Context, req GetStateRequest) (StateResponse, error)
	SetState(ctx context.Context, req SetStateRequest) error
	DeleteState(ctx context.Context, req DeleteStateRequest) error

	// GetReminder(ctx context.Context, req *GetReminderRequest) (*Reminder, error)
	// CreateReminder(ctx context.Context, req *CreateReminderRequest) error
	// DeleteReminder(ctx context.Context, req *DeleteReminderRequest) error
	// RenameReminder(ctx context.Context, req *RenameReminderRequest) error
	// CreateTimer(ctx context.Context, req *CreateTimerRequest) error
	// DeleteTimer(ctx context.Context, req *DeleteTimerRequest) error
}

type GetStateRequest struct {
	ActorType string
	ActorID   string
}

type StateResponse struct {
	Data []byte `json:"data"`
}

type SetStateRequest struct {
	ActorType string
	ActorID   string
	Value     []byte `json:"value"`
}

type DeleteStateRequest struct {
	ActorType string
	ActorID   string
}
