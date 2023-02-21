package actorsv2

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/dapr/components-contrib/state"
	"github.com/dapr/dapr/pkg/channel"
	configuration "github.com/dapr/dapr/pkg/config"
	daprCredentials "github.com/dapr/dapr/pkg/credentials"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	"github.com/dapr/dapr/pkg/resiliency"
)

const (
	stateKeySeparator = "||"
	stateKeySuffix    = "state"
)

type ActorStateStore interface {
	state.Store
	state.Transactioner
}

// ActorsOpts contains options for NewActors.
type ActorsOpts struct {
	AppID          string
	StateStore     ActorStateStore
	StateStoreName string
	AppChannel     channel.AppChannel
	CertChain      *daprCredentials.CertChain
	TracingSpec    configuration.TracingSpec
	Resiliency     resiliency.Provider
}

type actorsRuntime struct {
	appID       string
	appChannel  channel.AppChannel
	store       ActorStateStore
	storeName   string
	certChain   *daprCredentials.CertChain
	tracingSpec configuration.TracingSpec
	resiliency  resiliency.Provider
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewActors create a new actors runtime with given config.
func NewActors(opts ActorsOpts) Actors {
	ctx, cancel := context.WithCancel(context.Background())
	return &actorsRuntime{
		appID:       opts.AppID,
		appChannel:  opts.AppChannel,
		store:       opts.StateStore,
		storeName:   opts.StateStoreName,
		certChain:   opts.CertChain,
		tracingSpec: opts.TracingSpec,
		resiliency:  opts.Resiliency,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (a *actorsRuntime) Init() error {
	return nil
}

func (a *actorsRuntime) Close() error {
	return nil
}

func (a *actorsRuntime) GetState(ctx context.Context, req GetStateRequest) (StateResponse, error) {
	return StateResponse{}, nil
}

func (a *actorsRuntime) SetState(ctx context.Context, req SetStateRequest) error {
	return nil
}

func (a *actorsRuntime) DeleteState(ctx context.Context, req DeleteStateRequest) error {
	return nil
}

func (a *actorsRuntime) Call(parentCtx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	act := req.Actor()

	// Get a context that is canceled when this method returns
	// This is used by the store to cancel the transaction
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Start a state transaction that will retrieve the stateRes and acquire a lock
	stateKey := a.constructActorStateKey(act.GetActorId(), act.GetActorId(), stateKeySuffix)
	stateRes, commit, err := a.store.Transaction(ctx, state.TransactionStartRequest{
		Key: stateKey,
	})
	if err != nil {
		return nil, err
	}

	if len(stateRes.Data) > 0 {
		statepb, err := a.unserializeActorState(stateRes.Data)
		if err != nil {
			return nil, err
		}

		// Set the actor state
		req.WithActorState(statepb)
	}

	policyDef := a.resiliency.ActorPostLockPolicy(act.ActorType, act.ActorId)

	// If the request can be retried, we need to enable replaying
	if policyDef != nil && policyDef.HasRetries() {
		req.WithReplay(true)
	}

	// Invoke the method on the actor
	// In case of error, we return which causes the transaction to be canceled automatically
	policyRunner := resiliency.NewRunnerWithOptions(ctx, policyDef,
		resiliency.RunnerOpts[*invokev1.InvokeMethodResponse]{
			Disposer: resiliency.DisposerCloser[*invokev1.InvokeMethodResponse],
		},
	)
	resp, err := policyRunner(func(ctx context.Context) (*invokev1.InvokeMethodResponse, error) {
		return a.appChannel.InvokeActor(ctx, req)
	})
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, errors.New("error from actor service: response object is nil")
	}

	if resp.Status().Code != int32(codes.OK) {
		respData, _ := resp.RawDataFull()
		return nil, fmt.Errorf("error from actor service: %s", string(respData))
	}

	// Commit the transaction if everything is fine
	cr := state.TransactionCommitRequest{
		Key: stateKey,
	}
	if resp.ActorStateDelete() {
		cr.DeleteValue = true
	} else {
		cr.UpdateValue = resp.ActorStateUpdate()
	}
	err = commit(cr)
	if err != nil {
		return nil, fmt.Errorf("error committing state: %w", err)
	}

	return resp, nil
}

func (a *actorsRuntime) getAppChannel(actorType string) channel.AppChannel {
	return a.appChannel
}

func (a *actorsRuntime) constructActorStateKey(actorType, actorID, key string) string {
	return a.appID + stateKeySeparator +
		actorType + stateKeySeparator +
		actorID + stateKeySeparator +
		key
}

func (a *actorsRuntime) unserializeActorState(data []byte) (*structpb.Struct, error) {
	v := &structpb.Struct{}
	err := proto.Unmarshal(data, v)
	if err != nil {
		return nil, fmt.Errorf("failed to unserialize protobuf data: %w", err)
	}
	return v, nil
}

func (a *actorsRuntime) serializeActorState(v *structpb.Struct) ([]byte, error) {
	return proto.Marshal(v)
}
