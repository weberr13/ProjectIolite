package brain

import (
	"context"

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

func (m *MockThinker) Evaluate(ctx context.Context, sv SignVerifier, peerOutput Response, prev Decision) (Decision, error) {
	args := m.Called(ctx, sv, peerOutput, prev)
	return args.Get(0).(Decision), args.Error(1)
}
