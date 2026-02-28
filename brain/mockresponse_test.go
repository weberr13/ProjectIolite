package brain

import (
	"github.com/stretchr/testify/mock"
)

// MockResponse is now exported for use in gemini/claude tests
type MockResponse struct{ mock.Mock }

func (m *MockResponse) CoT() []Signed                   { return nil }
func (m *MockResponse) Text() *Signed                   { return nil }
func (m *MockResponse) Prompt() Signed                  { return Signed{} }
func (m *MockResponse) Describe(sv SignVerifier) string { return "" }
func (m *MockResponse) Sign(sv SignVerifier) error      { return nil }
func (m *MockResponse) Verify(sv SignVerifier) error    { return nil }
func (m *MockResponse) Source() string                  { return "mock" }
func (m *MockResponse) IsError() error {
	args := m.Called()
	return args.Error(0)
}
