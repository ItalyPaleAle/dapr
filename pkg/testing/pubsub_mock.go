package testing

import (
	"context"
	"sync"

	mock "github.com/stretchr/testify/mock"

	"github.com/dapr/components-contrib/pubsub"
)

// MockPubSub is a mock pub-sub component object.
type MockPubSub struct {
	mock.Mock
	handlers map[string]pubsub.Handler
	lock     sync.RWMutex
}

// Init is a mock initialization method.
func (m *MockPubSub) Init(metadata pubsub.Metadata) error {
	m.handlers = make(map[string]pubsub.Handler)
	m.lock = sync.RWMutex{}

	args := m.Called(metadata)
	return args.Error(0)
}

// Publish is a mock publish method.
func (m *MockPubSub) Publish(req *pubsub.PublishRequest) error {
	if req.Topic != "" {
		m.lock.RLock()
		handler, ok := m.handlers[req.Topic]
		if ok && handler != nil {
			msg := &pubsub.NewMessage{
				Data:        req.Data,
				Topic:       req.Topic,
				Metadata:    req.Metadata,
				ContentType: req.ContentType,
			}
			handler(context.Background(), msg)
		}
		m.lock.RUnlock()
	}

	args := m.Called(req)
	return args.Error(0)
}

// Subscribe is a mock subscribe method.
func (m *MockPubSub) Subscribe(req pubsub.SubscribeRequest, handler pubsub.Handler) error {
	if req.Topic != "" {
		m.lock.Lock()
		m.handlers[req.Topic] = handler
		m.lock.Unlock()
	}

	args := m.Called(req, handler)
	return args.Error(0)
}

func (m *MockPubSub) Close() error {
	return nil
}

func (m *MockPubSub) Features() []pubsub.Feature {
	return nil
}

type FailingPubsub struct {
	Failure Failure
}

func (f *FailingPubsub) Init(metadata pubsub.Metadata) error {
	return nil
}

func (f *FailingPubsub) Publish(req *pubsub.PublishRequest) error {
	return f.Failure.PerformFailure(req.Topic)
}

func (f *FailingPubsub) Subscribe(req pubsub.SubscribeRequest, handler pubsub.Handler) error {
	err := f.Failure.PerformFailure(req.Topic)
	if err != nil {
		return err
	}

	// This handler can also be calling into things that'll fail
	return handler(context.Background(), &pubsub.NewMessage{
		Topic: req.Topic,
		Metadata: map[string]string{
			"pubsubName": "failPubsub",
		},
		Data: []byte(req.Topic),
	})
}

func (f *FailingPubsub) Close() error {
	return nil
}

func (f *FailingPubsub) Features() []pubsub.Feature {
	return nil
}
