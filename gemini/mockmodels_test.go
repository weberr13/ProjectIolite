package gemini

import (
	"context"

	"github.com/stretchr/testify/mock"
	"google.golang.org/genai"
)

type MockModels struct {
	mock.Mock
}

func (m *MockModels) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	args := m.Called(ctx, model, contents, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*genai.GenerateContentResponse), args.Error(1)
}
