package universalapi

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapr/dapr/pkg/actors"
	"github.com/dapr/dapr/pkg/messages"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	runtimev1pb "github.com/dapr/dapr/pkg/proto/runtime/v1"
	"github.com/dapr/dapr/pkg/resiliency"
)

func (a *UniversalAPI) InvokeActorV2Alpha1(ctx context.Context, in *runtimev1pb.InvokeActorRequest) (*runtimev1pb.InvokeActorResponse, error) {
	response := &runtimev1pb.InvokeActorResponse{}

	if a.ActorV2 == nil {
		err := status.Errorf(codes.Internal, messages.ErrActorRuntimeNotFound)
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

	policyRunner := resiliency.NewRunnerWithOptions(ctx, policyDef,
		resiliency.RunnerOpts[*invokev1.InvokeMethodResponse]{
			Disposer: resiliency.DisposerCloser[*invokev1.InvokeMethodResponse],
		},
	)
	resp, err := policyRunner(func(ctx context.Context) (*invokev1.InvokeMethodResponse, error) {
		return a.ActorV2.Call(ctx, req)
	})
	if err != nil && !errors.Is(err, actors.ErrDaprResponseHeader) {
		err = status.Errorf(codes.Internal, messages.ErrActorInvoke, err)
		a.Logger.Debug(err)
		return response, err
	}

	if resp == nil {
		resp = invokev1.NewInvokeMethodResponse(500, "Blank request", nil)
	}
	defer resp.Close()

	response.Data, err = resp.RawDataFull()
	if err != nil {
		return nil, fmt.Errorf("failed to read response data: %w", err)
	}
	return response, nil
}
