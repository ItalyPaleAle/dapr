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

func (a *UniversalAPI) InvokeActorV2Alpha1(ctx context.Context, in *runtimev1pb.InvokeActorV2Alpha1Request) (*runtimev1pb.InvokeActorV2Alpha1Response, error) {
	res := &runtimev1pb.InvokeActorV2Alpha1Response{}

	if a.ActorV2 == nil {
		err := messages.ErrActorV2RuntimeNotFound
		a.Logger.Debug(err)
		return res, err
	}

	if in.AppId == "" {
		err := messages.ErrActorV2AppIDEmpty
		a.Logger.Debug(err)
		return res, err
	}

	// Create the InvokeMethodRequest object
	reqMetadata := make(map[string][]string, len(in.Metadata))
	for k, v := range in.Metadata {
		reqMetadata[k] = []string{v}
	}
	req := invokev1.NewInvokeMethodRequest(in.Method).
		WithActor(in.ActorType, in.ActorId).
		WithRawDataBytes(in.Data).
		WithMetadata(reqMetadata)
	defer req.Close()

	// If the target app is the same as appId, invoke the actor directly
	if in.AppId == a.AppID {
		err := a.invokeActorV2Alpha1Local(ctx, req, res)
		return res, err
	}

	// Resiliency is handled here for invocation.
	// Having the retry here allows us to capture that and be resilient to host failure.
	// Additionally, we don't perform timeouts at this level. This is because an actor
	// should technically wait forever on the locking mechanism. If we timeout while
	// waiting for the lock, we can also create a queue of calls that will try and continue
	// after the timeout.
	policyDef := a.Resiliency.ActorPreLockPolicy(in.ActorType, in.ActorId)

	if policyDef != nil {
		req.WithReplay(policyDef.HasRetries())
	}

	policyRunner := resiliency.NewRunnerWithOptions(ctx, policyDef,
		resiliency.RunnerOpts[*invokev1.InvokeMethodResponse]{
			Disposer: resiliency.DisposerCloser[*invokev1.InvokeMethodResponse],
		},
	)
	resp, err := policyRunner(func(ctx context.Context) (*invokev1.InvokeMethodResponse, error) {
		return a.ActorV2.Call(ctx, req)
	})
	if err != nil && !errors.Is(err, actors.ErrDaprResponseHeader) {
		err = messages.ErrActorV2Invoke.WithFormat(err.Error())
		a.Logger.Debug(err)
		return res, err
	}

	if resp == nil {
		resp = invokev1.NewInvokeMethodResponse(500, "Blank request", nil)
	}
	defer resp.Close()

	res.Data, err = resp.RawDataFull()
	if err != nil {
		return nil, fmt.Errorf("failed to read response data: %w", err)
	}
	return res, nil
}

func (a *UniversalAPI) invokeActorV2Alpha1Local(ctx context.Context, req *invokev1.InvokeMethodRequest, res *runtimev1pb.InvokeActorV2Alpha1Response) error {
	// We don't do resiliency here as it is handled in the API layer. See InvokeActor().
	resp, err := a.ActorV2.Call(ctx, req)
	if resp != nil {
		var readErr error
		defer resp.Close()
		res.Data, readErr = resp.RawDataFull()
		if readErr != nil {
			return fmt.Errorf("failed to read response data: %w", readErr)
		}
	}
	if err != nil {
		// We have to remove the error to keep the body, so callers must re-inspect for the header in the actual response.
		if resp != nil && errors.Is(err, actors.ErrDaprResponseHeader) {
			return nil
		}

		return status.Errorf(codes.Internal, messages.ErrActorInvoke, err)
	}
	return nil
}
