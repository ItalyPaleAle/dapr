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

package universalapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/dapr/components-contrib/state"
	"github.com/dapr/dapr/pkg/actors"
	"github.com/dapr/dapr/pkg/messages"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/dapr/dapr/pkg/resiliency"
)

func (a *UniversalAPI) RegisterActorTimer(ctx context.Context, in *runtimev1pb.RegisterActorTimerRequest) (*emptypb.Empty, error) {
	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	req := &actors.CreateTimerRequest{
		Name:      in.Name,
		ActorID:   in.ActorId,
		ActorType: in.ActorType,
		DueTime:   in.DueTime,
		Period:    in.Period,
		TTL:       in.Ttl,
		Callback:  in.Callback,
	}

	if in.Data != nil {
		j, err := json.Marshal(in.Data)
		if err != nil {
			return &emptypb.Empty{}, err
		}
		req.Data = j
	}
	err := a.Actors.CreateTimer(ctx, req)
	if err != nil {
		err = messages.ErrActorTimerCreate.WithFormat(err)
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *UniversalAPI) UnregisterActorTimer(ctx context.Context, in *runtimev1pb.UnregisterActorTimerRequest) (*emptypb.Empty, error) {
	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	req := &actors.DeleteTimerRequest{
		Name:      in.Name,
		ActorID:   in.ActorId,
		ActorType: in.ActorType,
	}

	err := a.Actors.DeleteTimer(ctx, req)
	if err != nil {
		err = messages.ErrActorTimerDelete.WithFormat(err)
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *UniversalAPI) RegisterActorReminder(ctx context.Context, in *runtimev1pb.RegisterActorReminderRequest) (*emptypb.Empty, error) {
	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	req := &actors.CreateReminderRequest{
		Name:      in.Name,
		ActorID:   in.ActorId,
		ActorType: in.ActorType,
		DueTime:   in.DueTime,
		Period:    in.Period,
		TTL:       in.Ttl,
	}

	if in.Data != nil {
		j, err := json.Marshal(in.Data)
		if err != nil {
			return &emptypb.Empty{}, err
		}
		req.Data = j
	}
	err := a.Actors.CreateReminder(ctx, req)
	if err != nil {
		err = messages.ErrActorReminderCreate.WithFormat(err)
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *UniversalAPI) UnregisterActorReminder(ctx context.Context, in *runtimev1pb.UnregisterActorReminderRequest) (*emptypb.Empty, error) {
	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	req := &actors.DeleteReminderRequest{
		Name:      in.Name,
		ActorID:   in.ActorId,
		ActorType: in.ActorType,
	}

	err := a.Actors.DeleteReminder(ctx, req)
	return &emptypb.Empty{}, err
}

func (a *UniversalAPI) RenameActorReminder(ctx context.Context, in *runtimev1pb.RenameActorReminderRequest) (*emptypb.Empty, error) {
	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	req := &actors.RenameReminderRequest{
		OldName:   in.OldName,
		ActorID:   in.ActorId,
		ActorType: in.ActorType,
		NewName:   in.NewName,
	}

	err := a.Actors.RenameReminder(ctx, req)
	if err != nil {
		err = messages.ErrActorReminderDelete.WithFormat(err)
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *UniversalAPI) GetActorState(ctx context.Context, in *runtimev1pb.GetActorStateRequest) (*runtimev1pb.GetActorStateResponse, error) {
	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return nil, err
	}

	hosted := a.Actors.IsActorHosted(ctx, &actors.ActorHostedRequest{
		ActorType: in.ActorType,
		ActorID:   in.ActorId,
	})

	if !hosted {
		err := messages.ErrActorInstanceMissing
		a.Logger.Debug(err)
		return nil, err
	}

	req := actors.GetStateRequest{
		ActorType: in.ActorType,
		ActorID:   in.ActorId,
		Key:       in.Key,
	}

	resp, err := a.Actors.GetState(ctx, &req)
	if err != nil {
		err = messages.ErrActorStateGet.WithFormat(err)
		a.Logger.Debug(err)
		return nil, err
	}

	return &runtimev1pb.GetActorStateResponse{
		Data: resp.Data,
	}, nil
}

func (a *UniversalAPI) ExecuteActorStateTransaction(ctx context.Context, in *runtimev1pb.ExecuteActorStateTransactionRequest) (*emptypb.Empty, error) {
	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	actorOps := []actors.TransactionalOperation{}

	for _, op := range in.Operations {
		var actorOp actors.TransactionalOperation
		switch op.OperationType {
		case string(state.OperationUpsert):
			setReq := map[string]any{
				"key":   op.Key,
				"value": op.Value.Value,
				// Actor state do not user other attributes from state request.
			}
			if meta := op.GetMetadata(); len(meta) > 0 {
				setReq["metadata"] = meta
			}

			actorOp = actors.TransactionalOperation{
				Operation: actors.Upsert,
				Request:   setReq,
			}
		case string(state.OperationDelete):
			delReq := map[string]interface{}{
				"key": op.Key,
				// Actor state do not user other attributes from state request.
			}

			actorOp = actors.TransactionalOperation{
				Operation: actors.Delete,
				Request:   delReq,
			}

		default:
			err := messages.ErrNotSupportedStateOperation.WithFormat(op.OperationType)
			a.Logger.Debug(err)
			return &emptypb.Empty{}, err
		}

		actorOps = append(actorOps, actorOp)
	}

	hosted := a.Actors.IsActorHosted(ctx, &actors.ActorHostedRequest{
		ActorType: in.ActorType,
		ActorID:   in.ActorId,
	})

	if !hosted {
		err := messages.ErrActorInstanceMissing
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	req := actors.TransactionalRequest{
		ActorID:    in.ActorId,
		ActorType:  in.ActorType,
		Operations: actorOps,
	}

	err := a.Actors.TransactionalStateOperation(ctx, &req)
	if err != nil {
		err = messages.ErrActorStateTransactionSave.WithFormat(err)
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (a *UniversalAPI) InvokeActor(ctx context.Context, in *runtimev1pb.InvokeActorRequest) (*runtimev1pb.InvokeActorResponse, error) {
	response := &runtimev1pb.InvokeActorResponse{}

	if a.Actors == nil {
		err := messages.ErrActorRuntimeNotFound
		a.Logger.Debug(err)
		return response, err
	}

	policyDef := a.Resiliency.ActorPreLockPolicy(in.ActorType, in.ActorId)

	reqMetadata := make(map[string][]string, len(in.Metadata))
	for k, v := range in.Metadata {
		reqMetadata[k] = []string{v}
	}
	req := invokev1.NewInvokeMethodRequest(in.Method).
		WithActor(in.ActorType, in.ActorId).
		WithRawDataBytes(in.Data).
		WithMetadata(reqMetadata)
	if policyDef != nil {
		req.WithReplay(policyDef.HasRetries())
	}
	defer req.Close()

	// Unlike other actor calls, resiliency is handled here for invocation.
	// This is due to actor invocation involving a lookup for the host.
	// Having the retry here allows us to capture that and be resilient to host failure.
	// Additionally, we don't perform timeouts at this level. This is because an actor
	// should technically wait forever on the locking mechanism. If we timeout while
	// waiting for the lock, we can also create a queue of calls that will try and continue
	// after the timeout.
	policyRunner := resiliency.NewRunnerWithOptions(ctx, policyDef,
		resiliency.RunnerOpts[*invokev1.InvokeMethodResponse]{
			Disposer: resiliency.DisposerCloser[*invokev1.InvokeMethodResponse],
		},
	)
	resp, err := policyRunner(func(ctx context.Context) (*invokev1.InvokeMethodResponse, error) {
		return a.Actors.Call(ctx, req)
	})
	if err != nil && !errors.Is(err, actors.ErrDaprResponseHeader) {
		err = messages.ErrActorInvoke.WithFormat(err)
		a.Logger.Debug(err)
		return response, err
	}

	if resp == nil {
		resp = invokev1.NewInvokeMethodResponse(500, "Blank response", nil)
	}
	defer resp.Close()

	response.Data, err = resp.RawDataFull()
	if err != nil {
		return nil, fmt.Errorf("failed to read response data: %w", err)
	}
	return response, nil
}
