package brain

import "github.com/stretchr/testify/mock"

// MockDecision is now exported
type MockDecision struct {
	mock.Mock
	setErr error
}

func (m *MockDecision) Cots() map[string][][]Signed  { return nil }
func (m *MockDecision) Prompts() map[string][]Signed { return nil }
func (m *MockDecision) Texts() map[string][]Signed {
	args := m.Called()
	return args.Get(0).(map[string][]Signed)
}
func (m *MockDecision) Sign(sv SignVerifier) error   { return nil }
func (m *MockDecision) Verify(sv SignVerifier) error { return nil }
func (m *MockDecision) IsError() error {
	if m.setErr != nil {
		return m.setErr
	}
	args := m.Called()
	return args.Error(0)
}
func (m *MockDecision) Add(s string, c []Signed, t Signed, sv SignVerifier) error { return nil }
func (m *MockDecision) SetError(err error) {
	m.setErr = err
}
func (m *MockDecision) SetAudits(Audits) {}
