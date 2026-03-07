package brain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func FuzzSanitizeJSON(f *testing.F) {
	// 1. Seed with known "Greebles"
	seeds := []string{
		`{"data": "valid\nnewline"}`,
		`{"data": "illegal\'quote"}`,
		`{"data": "nested\\\"quotes"}`,
		`{"data": "ampersand\&escape"}`,
		`{"data": "percent\%!(MISSING)greeble"}`,
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		wasValid := json.Valid(input)
		sanitized := SanitizeJSON(input)
		isNowValid := json.Valid(sanitized)

		if wasValid && !isNowValid {
			t.Errorf("[REGRESSION]: Sanitizer corrupted valid JSON\nIn: %s\nOut: %s", input, sanitized)
		}

		for i := 0; i < len(sanitized); i++ {
			if sanitized[i] == '\\' {
				if !isJSONValidEscape(sanitized, i) {
					t.Errorf("[FAILURE]: Illegal escape at index %d: %s", i, sanitized)
				} else {
					// JUMP OVER: Skip the escaped character to avoid re-auditing it.
					if i+1 < len(sanitized) && sanitized[i+1] == 'u' {
						i += 5 // Skip \uXXXX
					} else {
						i++ // Skip \n, \\, \", etc.
					}
				}
			}
		}

		if !wasValid && isNowValid {
			t.Logf("[SUCCESS]: Healed invalid JSON into valid format: %s", sanitized)
		}
	})
}

// helper to check if the byte slice contains a valid JSON escape
func isJSONValidEscape(b []byte, i int) bool {
	// If it's the last character, it's impossible for it to be a valid escape
	if i < 0 || i >= len(b)-1 || b[i] != '\\' {
		return false
	}

	// Check for \uXXXX (The "Forensic" Unicode escape)
	if b[i+1] == 'u' {
		if i+5 >= len(b) {
			return false
		} // Truncated \u
		for j := i + 2; j <= i+5; j++ {
			if !isHex(b[j]) {
				return false
			}
		}
		return true
	}

	return strings.ContainsRune(`"/\bfnrt\`, rune(b[i+1]))
}

func TestSigned_Flow(t *testing.T) {
	data := "Universal Genesis Prompt"
	namespace := TypeText
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
