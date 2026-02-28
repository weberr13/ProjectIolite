package brain

import "github.com/stretchr/testify/mock"

// MockDecision is now exported
type MockDecision struct{ mock.Mock }

func (m *MockDecision) Cots() map[string][][]Signed                               { return nil }
func (m *MockDecision) Prompts() map[string][]Signed                              { return nil }
func (m *MockDecision) Texts() map[string][]Signed                                { return nil }
func (m *MockDecision) Sign(sv SignVerifier) error                                { return nil }
func (m *MockDecision) Verify(sv SignVerifier) error                              { return nil }
func (m *MockDecision) Error() error                                              { return nil }
func (m *MockDecision) Add(s string, c []Signed, t Signed, sv SignVerifier) error { return nil }
