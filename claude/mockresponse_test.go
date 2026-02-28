package claude

import (
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
)

// MockResponse is now exported for use in gemini/claude tests
type MockResponse struct{ mock.Mock }

func (m *MockResponse) CoT() []brain.Signed { return nil }
func (m *MockResponse) Text() *brain.Signed {
	args := m.Called()
	return args.Get(0).(*brain.Signed)
}
func (m *MockResponse) Prompt() brain.Signed                  { return brain.Signed{} }
func (m *MockResponse) Describe(sv brain.SignVerifier) string { return "" }
func (m *MockResponse) Sign(sv brain.SignVerifier) error      { return nil }
func (m *MockResponse) Verify(sv brain.SignVerifier) error    { return nil }
func (m *MockResponse) Source() string                        { return "claude" }
