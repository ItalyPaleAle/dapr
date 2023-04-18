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
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/dapr/components-contrib/contenttype"
	contribMetadata "github.com/dapr/components-contrib/metadata"
	"github.com/dapr/components-contrib/state"
	stateLoader "github.com/dapr/dapr/pkg/components/state"
	diag "github.com/dapr/dapr/pkg/diagnostics"
	"github.com/dapr/dapr/pkg/encryption"
	"github.com/dapr/dapr/pkg/messages"
	commonv1pb "github.com/dapr/dapr/pkg/proto/common/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/dapr/dapr/pkg/resiliency"
)

func (a *UniversalAPI) SaveState(ctx context.Context, in *runtimev1pb.SaveStateRequest) (*emptypb.Empty, error) {
	store, err := a.getStateStore(in.StoreName)
	if err != nil {
		// Error has already been logged
		return &emptypb.Empty{}, err
	}

	l := len(in.States)
	if l == 0 {
		return &emptypb.Empty{}, nil
	}

	reqs := make([]state.SetRequest, l)
	for i, s := range in.States {
		var key string
		key, err = stateLoader.GetModifiedStateKey(s.Key, in.StoreName, a.AppID)
		if err != nil {
			err = messages.ErrStateBadKey.WithFormat(err)
			a.Logger.Debug(err)
			return &emptypb.Empty{}, messages.ErrStateBadKey.WithFormat(err)
		}
		req := state.SetRequest{
			Key:      key,
			Metadata: s.Metadata,
		}

		if req.Metadata[contribMetadata.ContentType] == contenttype.JSONContentType {
			err = json.Unmarshal(s.Value, &req.Value)
			if err != nil {
				return &emptypb.Empty{}, err
			}
		} else {
			req.Value = s.Value
		}

		if s.Etag != nil {
			req.ETag = &s.Etag.Value
		}
		if s.Options != nil {
			req.Options = state.SetStateOption{
				Consistency: StateConsistencyToString(s.Options.Consistency),
				Concurrency: StateConcurrencyToString(s.Options.Concurrency),
			}
		}
		if encryption.EncryptedStateStore(in.StoreName) {
			val, encErr := encryption.TryEncryptValue(in.StoreName, s.Value)
			if encErr != nil {
				a.Logger.Debug(encErr)
				return &emptypb.Empty{}, encErr
			}

			req.Value = val
		}

		reqs[i] = req
	}

	start := time.Now()
	policyRunner := resiliency.NewRunner[struct{}](ctx,
		a.Resiliency.ComponentOutboundPolicy(in.StoreName, resiliency.Statestore),
	)
	_, err = policyRunner(func(ctx context.Context) (struct{}, error) {
		// If there's a single request, perform it in non-bulk
		if len(reqs) == 1 {
			return struct{}{}, store.Set(ctx, &reqs[0])
		}
		return struct{}{}, store.BulkSet(ctx, reqs)
	})
	elapsed := diag.ElapsedSince(start)

	diag.DefaultComponentMonitoring.StateInvoked(ctx, in.StoreName, diag.Set, err == nil, elapsed)

	if err != nil {
		err = a.stateErrorResponse(err, messages.ErrStateSave, in.StoreName, err.Error())
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *UniversalAPI) DeleteState(ctx context.Context, in *runtimev1pb.DeleteStateRequest) (*emptypb.Empty, error) {
	store, err := a.getStateStore(in.StoreName)
	if err != nil {
		// Error has already been logged
		return &emptypb.Empty{}, err
	}

	key, err := stateLoader.GetModifiedStateKey(in.Key, in.StoreName, a.AppID)
	if err != nil {
		err = messages.ErrStateBadKey.WithFormat(err)
		a.Logger.Debug(err)
		return &emptypb.Empty{}, messages.ErrStateBadKey.WithFormat(err)
	}
	req := state.DeleteRequest{
		Key:      key,
		Metadata: in.Metadata,
	}
	if in.Etag != nil {
		req.ETag = &in.Etag.Value
	}
	if in.Options != nil {
		req.Options = state.DeleteStateOption{
			Concurrency: StateConcurrencyToString(in.Options.Concurrency),
			Consistency: StateConsistencyToString(in.Options.Consistency),
		}
	}

	start := time.Now()
	policyRunner := resiliency.NewRunner[any](ctx,
		a.Resiliency.ComponentOutboundPolicy(in.StoreName, resiliency.Statestore),
	)
	_, err = policyRunner(func(ctx context.Context) (any, error) {
		return nil, store.Delete(ctx, &req)
	})
	elapsed := diag.ElapsedSince(start)

	diag.DefaultComponentMonitoring.StateInvoked(ctx, in.StoreName, diag.Delete, err == nil, elapsed)

	if err != nil {
		err = a.stateErrorResponse(err, messages.ErrStateDelete, in.Key, err.Error())
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

func (a *UniversalAPI) DeleteBulkState(ctx context.Context, in *runtimev1pb.DeleteBulkStateRequest) (*emptypb.Empty, error) {
	store, err := a.getStateStore(in.StoreName)
	if err != nil {
		// Error has already been logged
		return &emptypb.Empty{}, err
	}

	reqs := make([]state.DeleteRequest, 0, len(in.States))
	for _, item := range in.States {
		var key string
		key, err = stateLoader.GetModifiedStateKey(item.Key, in.StoreName, a.AppID)
		if err != nil {
			err = messages.ErrStateBadKey.WithFormat(err)
			a.Logger.Debug(err)
			return &emptypb.Empty{}, messages.ErrStateBadKey.WithFormat(err)
		}
		req := state.DeleteRequest{
			Key:      key,
			Metadata: item.Metadata,
		}
		if item.Etag != nil {
			req.ETag = &item.Etag.Value
		}
		if item.Options != nil {
			req.Options = state.DeleteStateOption{
				Concurrency: StateConcurrencyToString(item.Options.Concurrency),
				Consistency: StateConsistencyToString(item.Options.Consistency),
			}
		}
		reqs = append(reqs, req)
	}

	start := time.Now()
	policyRunner := resiliency.NewRunner[any](ctx,
		a.Resiliency.ComponentOutboundPolicy(in.StoreName, resiliency.Statestore),
	)
	_, err = policyRunner(func(ctx context.Context) (any, error) {
		return nil, store.BulkDelete(ctx, reqs)
	})
	elapsed := diag.ElapsedSince(start)

	diag.DefaultComponentMonitoring.StateInvoked(ctx, in.StoreName, diag.BulkDelete, err == nil, elapsed)

	if err != nil {
		a.Logger.Debug(err)
		return &emptypb.Empty{}, err
	}
	return &emptypb.Empty{}, nil
}

// stateErrorResponse takes a state store error, format and args and returns a status code encoded gRPC error.
// Although we take an APIError, we override the status codes.
func (a *UniversalAPI) stateErrorResponse(err error, msg messages.APIError, arg any) error {
	e, ok := err.(*state.ETagError)
	if !ok {
		return status.Errorf(codes.Internal, format, args...)
	}
	switch e.Kind() {
	case state.ETagMismatch:
		return status.Errorf(codes.Aborted, format, args...)
	case state.ETagInvalid:
		return status.Errorf(codes.InvalidArgument, format, args...)
	default:
		return status.Errorf(codes.Internal, format, args...)
	}
}

//nolint:nosnakecase
func StateConsistencyToString(c commonv1pb.StateOptions_StateConsistency) string { //nolint:nosnakecase
	switch c {
	case commonv1pb.StateOptions_CONSISTENCY_EVENTUAL:
		return "eventual"
	case commonv1pb.StateOptions_CONSISTENCY_STRONG:
		return "strong"
	default:
		return ""
	}
}

//nolint:nosnakecase
func StateConcurrencyToString(c commonv1pb.StateOptions_StateConcurrency) string {
	switch c {
	case commonv1pb.StateOptions_CONCURRENCY_FIRST_WRITE:
		return "first-write"
	case commonv1pb.StateOptions_CONCURRENCY_LAST_WRITE:
		return "last-write"
	default:
		return ""
	}
}

func (a *UniversalAPI) getStateStore(name string) (state.Store, error) {
	if a.CompStore.StateStoresLen() == 0 {
		err := messages.ErrStateStoresNotConfigured
		a.Logger.Debug(err)
		return nil, err
	}

	state, ok := a.CompStore.GetStateStore(name)
	if !ok {
		err := messages.ErrStateStoreNotFound.WithFormat(name)
		a.Logger.Debug(err)
		return nil, err
	}

	return state, nil
}
