package gemini

import (
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
)

// MockDecision is now exported
type MockDecision struct{ mock.Mock }

func (m *MockDecision) Cots() map[string][][]brain.Signed  { return nil }
func (m *MockDecision) Prompts() map[string][]brain.Signed { return nil }
func (m *MockDecision) Texts() map[string][]brain.Signed   { return nil }
func (m *MockDecision) Sign(sv brain.SignVerifier) error   { return nil }
func (m *MockDecision) Verify(sv brain.SignVerifier) error { return nil }
func (m *MockDecision) IsError() error                     { return nil }
func (m *MockDecision) Add(s string, c []brain.Signed, t brain.Signed, sv brain.SignVerifier) error {
	args := m.Called(s, c, t, sv)
	return args.Error(0)
}
func (m *MockDecision) SetError(err error) {}
