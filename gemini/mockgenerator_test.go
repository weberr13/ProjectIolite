package gemini

import (
	"context"

	"github.com/stretchr/testify/mock"
	"google.golang.org/genai"
)

// MockGenerator satisfies the ContentGenerator sliver interface
type MockGenerator struct {
	mock.Mock
}

func (m *MockGenerator) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	args := m.Called(ctx, model, contents, config)
	return args.Get(0).(*genai.GenerateContentResponse), args.Error(1)
}
