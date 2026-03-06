package brain

import (
	"context"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/mock"
)

type MockThinker struct {
	mock.Mock
}

func (m *MockThinker) Think(ctx context.Context, sv SignVerifier, input Request) (Response, error) {
	args := m.Called(ctx, sv, input)
	// Safe assertion: if index 0 is nil, return nil (though we should avoid this in tests)
	resp, _ := args.Get(0).(Response)
	return resp, args.Error(1)
}

func (m *MockThinker) Evaluate(ctx context.Context, sv SignVerifier, peerOutput Response) (Decision, error) {
	args := m.Called(ctx, sv, peerOutput)
	return args.Get(0).(Decision), args.Error(1)
}

func (*MockThinker) Dream(_ context.Context, _ *openapi3.T, _ SignVerifier) (Hydration, error) {
	return Hydration{}, nil
}
