package gemini

import "github.com/stretchr/testify/mock"

// MockSignVerifier handles the interface injection for the test context
type MockSignVerifier struct {
	mock.Mock
}

func (m *MockSignVerifier) Sign(data string) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}

func (m *MockSignVerifier) Verify(data, signature string, publicKey ...string) error {
	args := m.Called(data, signature)
	return args.Error(0)
}

func (m *MockSignVerifier) ExportPublicKey() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSignVerifier) Alg() string {
	args := m.Called()
	return args.String()
}

func (m *MockSignVerifier) VerifyPy() string {
	args := m.Called()
	return args.String()
}
