package brain

import (
	"github.com/stretchr/testify/mock"
)

// MockResponse is now exported for use in gemini/claude tests
type MockResponse struct{ mock.Mock }

func (m *MockResponse) CoT(sv SignVerifier) []Signed {
	args := m.Called(sv)
	return args.Get(0).([]Signed)
}

func (m *MockResponse) Text() *Signed {
	args := m.Called()
	return args.Get(0).(*Signed)
}

func (m *MockResponse) Prompt() Signed {
	args := m.Called()
	return args.Get(0).(Signed)
}
func (m *MockResponse) Describe(sv SignVerifier) string { return "" }
func (m *MockResponse) Sign(sv SignVerifier) error {
	args := m.Called(sv)
	return args.Error(0)
}

func (m *MockResponse) Verify(sv SignVerifier) error {
	args := m.Called(sv)
	return args.Error(0)
}

func (m *MockResponse) Source() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockResponse) IsError() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockResponse) SetError(err error) {
}
