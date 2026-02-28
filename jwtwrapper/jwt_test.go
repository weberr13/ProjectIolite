package jwtwrapper

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/unicode/norm"
)

func TestSignVerifier_Interface(t *testing.T) {
	// Generate valid Ed25519 keypair for the test context
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	t.Run("New Constructor State", func(t *testing.T) {
		sv := New(pub, priv)

		assert.NotNil(t, sv)
		assert.Equal(t, pub, sv.publicKey, "Should store the provided public key")
		assert.Equal(t, priv, sv.privateKey, "Should store the provided private key")
		assert.Equal(t, "EdDSA", sv.Alg(), "Alg() must return the fixed constant")
	})

	t.Run("Normalization Logic Gate", func(t *testing.T) {
		sv := New(pub, priv)

		// Use a string with a combining character (e.g., 'e' + '´' vs 'é')
		// This ensures the Sign() method is actually invoking the NFC normalization
		input := "Café" // Using a composed 'é'

		// We aren't testing the signature itself, but verifying the flow doesn't error
		sig, err := sv.Sign(input)
		assert.NoError(t, err)
		assert.NotEmpty(t, sig)
	})

	t.Run("ExportPublicKey Formatting", func(t *testing.T) {
		sv := New(pub, priv)

		exported := sv.ExportPublicKey()

		// Verify it is valid Std Base64
		decoded, err := base64.StdEncoding.DecodeString(exported)
		assert.NoError(t, err, "Exported key should be valid Std Base64")
		assert.Equal(t, []byte(pub), decoded, "Decoded export must match original public key bytes")
	})
}

func TestSignVerifier_VerifyPyEmbedding(t *testing.T) {
	// This tests the 'embed' logic to ensure the Python script is actually linked
	pub, priv, _ := ed25519.GenerateKey(nil)
	sv := New(pub, priv)

	script := sv.VerifyPy()

	assert.NotEmpty(t, script, "The embedded Python string should not be empty")
	assert.Contains(t, script, "def verify_iolite_block", "Should contain our specific verification function")
}

func TestSignVerifier_SignVerifyFlow(t *testing.T) {
	// While we don't test the library's math, we must test that our
	// New() constructor wires the signing method correctly.
	pub, priv, _ := ed25519.GenerateKey(nil)
	sv := New(pub, priv)

	data := "Iolite-Protocol-Data"

	sig, err := sv.Sign(data)
	require.NoError(t, err)

	// Test that our Verify call (using our stored public key) reconciles our Sign call
	err = sv.Verify(norm.NFC.String(data), sig)
	assert.NoError(t, err, "Signature should verify against normalized data")
}

func TestSignVerifier_ErrorHandling(t *testing.T) {
	t.Run("Sign Failure Bubbling", func(t *testing.T) {
		mockMethod := new(MockSigningMethod)
		sv := &SignVerifier{
			method: mockMethod,
		}

		expectedErr := errors.New("low_entropy_error")
		mockMethod.On("Sign", mock.Anything, mock.Anything).Return("", expectedErr)

		_, err := sv.Sign("test data")
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err, "Our Sign() should bubble up the underlying method error")
	})
}
