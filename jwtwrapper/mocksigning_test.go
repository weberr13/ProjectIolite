package jwtwrapper

import "github.com/stretchr/testify/mock"

// MockSigningMethod allows us to simulate library-level failures
type MockSigningMethod struct {
	mock.Mock
}

func (m *MockSigningMethod) Alg() string { return "EdDSA" }

func (m *MockSigningMethod) Sign(signingString string, key interface{}) (string, error) {
	args := m.Called(signingString, key)
	return args.String(0), args.Error(1)
}

func (m *MockSigningMethod) Verify(signingString, signature string, key interface{}) error {
	args := m.Called(signingString, signature, key)
	return args.Error(0)
}
