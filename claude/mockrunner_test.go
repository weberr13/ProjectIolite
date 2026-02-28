package claude

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockRunner struct {
	mock.Mock
}

func (m *MockRunner) Run(ctx context.Context, code string) (string, error) {
	args := m.Called(ctx, code)
	return args.String(0), args.Error(1)
}
