package actorsv2

import (
	"context"

	"github.com/dapr/components-contrib/state"
	"github.com/dapr/dapr/pkg/channel"
	configuration "github.com/dapr/dapr/pkg/config"
	daprCredentials "github.com/dapr/dapr/pkg/credentials"
	invokev1 "github.com/dapr/dapr/pkg/messaging/v1"
	"github.com/dapr/dapr/pkg/resiliency"
	"google.golang.org/grpc"
)

// GRPCConnectionFn is the type of the function that returns a gRPC connection
type GRPCConnectionFn func(ctx context.Context, address string, id string, namespace string, customOpts ...grpc.DialOption) (*grpc.ClientConn, func(destroy bool), error)

type ActorStateStore interface {
	state.Store
	state.Transactioner
}

// ActorsOpts contains options for NewActors.
type ActorsOpts struct {
	StateStore       ActorStateStore
	StateStoreName   string
	AppChannel       channel.AppChannel
	GRPCConnectionFn GRPCConnectionFn
	CertChain        *daprCredentials.CertChain
	TracingSpec      configuration.TracingSpec
	Resiliency       resiliency.Provider
}

type actorsRuntime struct {
	appChannel       channel.AppChannel
	store            ActorStateStore
	storeName        string
	grpcConnectionFn GRPCConnectionFn
	certChain        *daprCredentials.CertChain
	tracingSpec      configuration.TracingSpec
	resiliency       resiliency.Provider
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewActors create a new actors runtime with given config.
func NewActors(opts ActorsOpts) Actors {
	ctx, cancel := context.WithCancel(context.Background())
	return &actorsRuntime{
		appChannel:       opts.AppChannel,
		store:            opts.StateStore,
		storeName:        opts.StateStoreName,
		grpcConnectionFn: opts.GRPCConnectionFn,
		certChain:        opts.CertChain,
		tracingSpec:      opts.TracingSpec,
		resiliency:       opts.Resiliency,
		ctx:              ctx,
		cancel:           cancel,
	}
}

func (a *actorsRuntime) Init() error {
	return nil
}

func (a *actorsRuntime) Close() error {
	return nil
}

func (a *actorsRuntime) Call(ctx context.Context, req *invokev1.InvokeMethodRequest) (*invokev1.InvokeMethodResponse, error) {
	return nil, nil
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
