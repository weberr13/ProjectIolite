package claude

import (
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
)

// MockResponse is now exported for use in gemini/claude tests
type MockResponse struct{ mock.Mock }

func (m *MockResponse) CoT(sv brain.SignVerifier) []brain.Signed {
	args := m.Called(sv)
	return args.Get(0).([]brain.Signed)
}

func (m *MockResponse) Text() *brain.Signed {
	args := m.Called()
	return args.Get(0).(*brain.Signed)
}

func (m *MockResponse) Prompt() brain.Signed {
	args := m.Called()
	return args.Get(0).(brain.Signed)
}
func (m *MockResponse) Describe(sv brain.SignVerifier) string { return "" }
func (m *MockResponse) Sign(sv brain.SignVerifier) error      { return nil }
func (m *MockResponse) Verify(sv brain.SignVerifier) error {
	args := m.Called(sv)
	return args.Error(0)
}

func (m *MockResponse) Source() string {
	args := m.Called()
	return args.Get(0).(string)
}

func (m *MockResponse) IsError() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockResponse) SetError(err error) {
}
