package brain

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSigned_Flow(t *testing.T) {
	data := "Universal Genesis Prompt"
	namespace := "iolite.core"
	b64Data := base64.StdEncoding.EncodeToString([]byte(data))

	t.Run("Happy Path: New -> Sign -> Verify", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		s := NewUnsigned(data, namespace)
		expectedSig := "sig_alpha_123"

		// Step 1: Sign
		// The data passed to sv.Sign must be B64(data) + PrevSignature ("")
		mockSV.On("Sign", b64Data+"").Return(expectedSig, nil).Once()
		err := s.Sign(mockSV)

		assert.NoError(t, err)
		assert.Equal(t, expectedSig, s.Signature)

		// Step 2: Verify
		mockSV.On("Verify", b64Data+"", expectedSig).Return(nil).Once()
		err = s.Verify(mockSV)
		assert.NoError(t, err)
	})

	t.Run("Idempotency: Sign Twice Triggers Verify", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		s := Signed{
			Data:      data,
			Signature: "existing_sig",
		}

		// If s.Signature != "", Sign() must call Verify() instead of Sign()
		mockSV.On("Verify", b64Data+"", "existing_sig").Return(nil).Once()

		err := s.Sign(mockSV)
		assert.NoError(t, err, "Signing an already signed block should just verify it")
		mockSV.AssertNotCalled(t, "Sign", mock.Anything)
	})

	t.Run("Chain Logic: NextUnsigned Continuity", func(t *testing.T) {
		parent := Signed{
			Namespace: namespace,
			Signature: "parent_sig",
		}

		childData := "Sub-thread logic"
		child := parent.NextUnsigned(childData)

		assert.Equal(t, namespace, child.Namespace)
		assert.Equal(t, "parent_sig", child.PrevSignature)
		assert.Empty(t, child.Signature, "Child should start unsigned")
	})

	t.Run("Error Branch: Unsigned Verification", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		s := NewUnsigned(data, namespace)
		err := s.Verify(mockSV)

		assert.ErrorIs(t, err, ErrUnsigned)
	})

	t.Run("Error Branch: Signing Failure Propagation", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		s := NewUnsigned(data, namespace)
		signErr := errors.New("entropy_exhausted")

		mockSV.On("Sign", mock.Anything).Return("", signErr).Once()

		err := s.Sign(mockSV)
		assert.Equal(t, signErr, err)
		assert.Empty(t, s.Signature, "Signature should remain empty on failure")
	})
}
