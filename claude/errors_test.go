package claude

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClaudeError_Logic(t *testing.T) {
	rootErr := errors.New("anthropic_api_timeout")
	input := "Check the manifest"

	t.Run("Describe: Lazy Signing Safety", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		ce := &ClaudeError{e: rootErr, input: input}

		// If sig is empty, Describe should attempt to sign the error text
		mockSV.On("Sign", rootErr.Error()).Return("sig_lazy_123", nil).Once()

		desc := ce.Describe(mockSV)

		assert.Contains(t, desc, rootErr.Error())
		assert.Equal(t, "sig_lazy_123", ce.sig)
		mockSV.AssertExpectations(t)
	})

	t.Run("Error/Unwrap: Idiomatic Compliance", func(t *testing.T) {
		ce := &ClaudeError{e: rootErr, input: input}

		// Check the string format
		assert.Equal(t, `claude: anthropic_api_timeout (input: "Check the manifest")`, ce.Error())

		// Check the Unwrap chain
		assert.True(t, errors.Is(ce, rootErr), "errors.Is failed to find root cause")

		var target *ClaudeError
		assert.True(t, errors.As(ce, &target), "errors.As failed to extract ClaudeError")
	})

	t.Run("Verify: Internal vs External State", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		ce := &ClaudeError{e: rootErr}

		// Case 1: Internal state (unsinged) is OK
		ce.sig = ""
		assert.NoError(t, ce.Verify(mockSV))

		// Case 2: Explicitly signed must be verifiable
		ce.sig = "external_sig"
		mockSV.On("Verify", rootErr.Error(), "external_sig").Return(nil).Once()
		assert.NoError(t, ce.Verify(mockSV))
	})
}

func TestClaudeError_DeepCoverage(t *testing.T) {
	rootErr := errors.New("anthropic_overloaded")
	inputPrompt := "Analyze this trace"

	t.Run("Sign: Trigger Internal Error Branch (line 19)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		ce := &ClaudeError{e: rootErr}

		// THE TARGET: Simulate a failure in the Signer (e.g. HSM timeout)
		signErr := errors.New("hsm_not_responding")
		mockSV.On("Sign", rootErr.Error()).Return("", signErr).Once()

		err := ce.Sign(mockSV)

		// ASSERT: The error bubbles up and state remains clean
		assert.ErrorIs(t, err, signErr)
		assert.Empty(t, ce.sig)
		mockSV.AssertExpectations(t)
	})

	t.Run("CoT and Text: Verification of Signed Wrappers", func(t *testing.T) {
		ce := &ClaudeError{e: rootErr}

		// Test CoT() slice construction
		cot := ce.CoT()
		assert.Len(t, cot, 1)
		assert.Equal(t, rootErr.Error(), cot[0].Data)
		assert.Equal(t, "cot", cot[0].Namespace)

		// Test Text() pointer return
		txt := ce.Text()
		assert.NotNil(t, txt)
		assert.Equal(t, rootErr.Error(), txt.Data)
		assert.Equal(t, "text", txt.Namespace)
	})

	t.Run("Prompt and Source: Identity Compliance", func(t *testing.T) {
		ce := &ClaudeError{e: rootErr, input: inputPrompt}

		// Test Prompt() construction
		p := ce.Prompt()
		assert.Equal(t, inputPrompt, p.Data)
		assert.Equal(t, "prompt", p.Namespace)

		// Test Source() identification
		assert.Equal(t, "claude", ce.Source())
	})
}

func TestClaudeError_SignSuccess(t *testing.T) {
	t.Run("Sign: Happy Path State Persistence (lines 22-23)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		innerErr := errors.New("anthropic_rate_limit")
		ce := &ClaudeError{e: innerErr}

		// 1. Setup the expected success
		expectedSig := "valid_hsm_signature_001"
		mockSV.On("Sign", innerErr.Error()).Return(expectedSig, nil).Once()

		// 2. Execution
		err := ce.Sign(mockSV)

		// 3. ASSERTIONS
		assert.NoError(t, err)
		assert.Equal(t, expectedSig, ce.sig, "The happy path failed to persist the signature to e.sig")

		// 4. Verification: Now that it's signed, Verify() should use it
		mockSV.On("Verify", innerErr.Error(), expectedSig).Return(nil).Once()
		vErr := ce.Verify(mockSV)
		assert.NoError(t, vErr)

		mockSV.AssertExpectations(t)
	})
}
