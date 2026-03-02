package gemini

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGeminiError_Interface(t *testing.T) {
	innerErr := errors.New("api_quota_exceeded")
	inputPrompt := "Tell me a story"

	t.Run("Sign: Cryptographic Locking of Error Message", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		ge := &GeminiError{e: innerErr, input: inputPrompt}

		// Expectation: Signing the error string
		expectedSig := "sig_error_123"
		mockSV.On("Sign", innerErr.Error()).Return(expectedSig, nil).Once()

		err := ge.Sign(mockSV)

		assert.NoError(t, err)
		assert.Equal(t, expectedSig, ge.sig)
		mockSV.AssertExpectations(t)
	})

	t.Run("Describe: Lazy Signing Logic", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		ge := &GeminiError{e: innerErr, input: inputPrompt} // sig is empty

		// When Describe is called, if sig is empty, it attempts to sign
		mockSV.On("Sign", innerErr.Error()).Return("lazy_sig", nil).Once()

		desc := ge.Describe(mockSV)

		assert.Contains(t, desc, innerErr.Error())
		assert.Equal(t, "lazy_sig", ge.sig, "Describe should have lazily populated the signature")
	})

	t.Run("Verify: Internal State vs Explicit Audit", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		ge := &GeminiError{e: innerErr}

		// 1. Unsigned state is "ok" (Internal program state)
		ge.sig = ""
		err := ge.Verify(mockSV)
		assert.NoError(t, err)

		// 2. Signed state must actually be verified
		ge.sig = "manual_sig"
		mockSV.On("Verify", innerErr.Error(), "manual_sig").Return(nil).Once()
		err = ge.Verify(mockSV)
		assert.NoError(t, err)

		// 3. Corrupt signature detection
		badErr := errors.New("sig_mismatch")
		mockSV.On("Verify", innerErr.Error(), "manual_sig").Return(badErr).Once()
		err = ge.Verify(mockSV)
		assert.ErrorIs(t, err, badErr)
	})

	t.Run("Getters: Response Interface Compliance", func(t *testing.T) {
		ge := &GeminiError{e: innerErr, input: inputPrompt}

		assert.Equal(t, "gemini", ge.Source())

		text := ge.Text()
		assert.Equal(t, innerErr.Error(), text.Data)
		assert.Equal(t, "text", text.Namespace)

		prompt := ge.Prompt()
		assert.Equal(t, inputPrompt, prompt.Data)
		assert.Equal(t, "prompt", prompt.Namespace)
	})
}

func TestGeminiError_Unwrap(t *testing.T) {
	t.Run("Unwrap: Support errors.Is and errors.As", func(t *testing.T) {
		rootErr := errors.New("api_limit")
		ge := &GeminiError{e: rootErr, input: "test prompt"}

		// 1. Verify errors.Is works
		assert.True(t, errors.Is(ge, rootErr), "errors.Is should find the root error")

		// 2. Verify errors.As works to get the metadata
		var target *GeminiError
		assert.True(t, errors.As(ge, &target), "errors.As should extract the GeminiError struct")
		assert.Equal(t, "test prompt", target.input)
	})
}

func TestGeminiError_DeepCoverage(t *testing.T) {
	innerErr := errors.New("api_quota_exceeded")
	inputPrompt := "Generate a complex analysis"

	t.Run("CoT: Verify Unsigned Slice Construction", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Sign", mock.Anything).Return("sig_1", nil).Maybe()
		ge := &GeminiError{e: innerErr}

		cot := ge.CoT(mockSV)

		// ASSERT: The CoT should contain exactly one element wrapping the error string
		assert.Len(t, cot, 1)
		assert.Equal(t, innerErr.Error(), cot[0].Data)
		assert.Equal(t, "cot", cot[0].Namespace)
		assert.Equal(t, cot[0].Signature, "sig_1", "CoT should be Sigined! by default")
	})

	t.Run("Error: Verify Formatted String and Quoting", func(t *testing.T) {
		ge := &GeminiError{e: innerErr, input: inputPrompt}

		got := ge.Error()
		// %q adds quotes and escapes special characters
		expected := `gemini: api_quota_exceeded (input: "Generate a complex analysis")`

		assert.Equal(t, expected, got)
	})

	t.Run("Sign: Trigger Signing Error Branch (line 19)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		ge := &GeminiError{e: innerErr}

		// THE TARGET: Simulate a failure in the Signer (e.g. KMS timeout)
		signErr := errors.New("kms_unavailable")
		mockSV.On("Sign", innerErr.Error()).Return("", signErr).Once()

		err := ge.Sign(mockSV)

		// ASSERT: The error at line 19 is bubbled up immediately
		assert.ErrorIs(t, err, signErr)
		assert.Empty(t, ge.sig, "Signature should not be set on failure")

		mockSV.AssertExpectations(t)
	})
}
