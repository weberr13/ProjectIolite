package gemini

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
)

// Helper for cryptographic grounding
func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestGeminiDecision_New(t *testing.T) {
	t.Run("NewDecision: Pointer Dereference and Init Safety", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockResp := new(MockResponse)

		// Set up a valid initial response
		validText := &brain.Signed{Data: "Gemini output", Signature: "sig_g"}
		mockResp.On("Text").Return(validText).Maybe()
		mockResp.On("Source").Return("gemini").Maybe()
		mockResp.On("CoT").Return([]brain.Signed{}).Maybe()
		mockResp.On("Prompt").Return(brain.Signed{Signature: "sig_p"}).Maybe()

		// NewDecision verifies input first
		mockResp.On("Verify", mockSV).Return(nil).Once()

		// Signer expectations for the internal d.Sign(sv) call
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockSV.On("Sign", mock.Anything).Return("internal_sig", nil).Maybe()

		decision, err := NewDecision(mockResp, mockSV)

		assert.NoError(t, err)
		assert.NotNil(t, decision)
		assert.Equal(t, "Gemini output", decision.AllTexts["gemini"][0].Data)
	})

	t.Run("NewDecision: Verify Failure Returns Nil", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockResp := new(MockResponse)

		// Force corruption error
		corruptErr := errors.New("initial_response_tampered")
		mockResp.On("Verify", mockSV).Return(corruptErr).Once()

		decision, err := NewDecision(mockResp, mockSV)

		assert.ErrorIs(t, err, corruptErr)
		assert.Nil(t, decision)
	})
}

func TestGeminiDecision_AddAndStitch(t *testing.T) {
	t.Run("Add: Atomic Signing and PrevSignature Stitching", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			ChainOfThoughts: make(map[string][][]brain.Signed),
			AllPrompts:      make(map[string][]brain.Signed),
			AllTexts: map[string][]brain.Signed{
				"gemini": {{Data: "previous text", Signature: "sig_prev"}},
			},
		}

		source := "gemini"
		newText := brain.Signed{Data: "current text"} // Unsigned

		// 1. EXPECTATION: Verify the EXISTING block (The one we missed!)
		// The input for a first block is just B64(data) + ""
		mockSV.On("Verify", b64("previous text")+"", "sig_prev").Return(nil).Maybe()

		// 2. EXPECTATION: Sign the NEW block (with the prev signature salt)
		expectedInput := b64("current text") + "sig_prev"
		mockSV.On("Sign", expectedInput).Return("sig_curr", nil).Once()

		// 3. EXPECTATION: Verify the NEW block (part of the final d.Verify call)
		mockSV.On("Verify", expectedInput, "sig_curr").Return(nil).Once()

		err := d.Add(source, nil, newText, mockSV)

		assert.NoError(t, err)
		assert.Equal(t, "sig_prev", d.AllTexts[source][1].PrevSignature)
		assert.Equal(t, "sig_curr", d.AllTexts[source][1].Signature)

		mockSV.AssertExpectations(t)
	})
}

func TestGeminiDecision_RecursiveVerify(t *testing.T) {
	t.Run("Verify: Exhaustive Map Coverage", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"claude": {{{Data: "peer thought", Signature: "sig_c"}}},
			},
			AllTexts: map[string][]brain.Signed{
				"gemini": {
					{Data: "v1", Signature: "sig_v1"},
					{Data: "v2", Signature: "sig_v2"}, // Corrupt target
				},
			},
		}

		// Set up success for first two, failure for third
		mockSV.On("Verify", b64("peer thought")+"", "sig_c").Return(nil).Once()
		mockSV.On("Verify", b64("v1")+"", "sig_v1").Return(nil).Once()

		tamperErr := errors.New("signature_mismatch")
		mockSV.On("Verify", b64("v2")+"", "sig_v2").Return(tamperErr).Once()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, tamperErr)
	})

	t.Run("Verify: Fail Fast on Missing Signatures", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			AllPrompts: map[string][]brain.Signed{
				"gemini": {{Data: "prompt", Signature: ""}}, // Missing
			},
		}

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, brain.ErrUnsigned)
		mockSV.AssertNotCalled(t, "Verify", mock.Anything, mock.Anything)
	})
}

func TestGeminiDecision_Getters(t *testing.T) {
	t.Run("Cots: Deep Copy Isolation", func(t *testing.T) {
		d := &GeminiDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"gemini": {{{Data: "thought", Signature: "sig"}}},
			},
		}

		copy := d.Cots()
		copy["gemini"][0][0].Data = "MUTATED"

		assert.Equal(t, "thought", d.ChainOfThoughts["gemini"][0][0].Data)
	})
}

func TestGeminiDecision_GetterIsolation(t *testing.T) {
	t.Run("Prompts: Slice Backing Array Isolation", func(t *testing.T) {
		source := "gemini"
		d := &GeminiDecision{
			AllPrompts: map[string][]brain.Signed{
				source: {{Data: "original prompt", Signature: "sig_p"}},
			},
		}

		// 1. Get the clone
		exported := d.Prompts()

		// 2. Mutate the clone's slice (Modify existing element)
		exported[source][0].Data = "MUTATED"

		// 3. Mutate the clone's slice (Append new element)
		exported[source] = append(exported[source], brain.Signed{Data: "ghost prompt"})

		// ASSERT: The internal state of d remains "Grounded"
		assert.Equal(t, "original prompt", d.AllPrompts[source][0].Data, "Shallow copy detected: Mutation leaked to original")
		assert.Len(t, d.AllPrompts[source], 1, "Shallow copy detected: Append leaked to original")
	})

	t.Run("Texts: Map Key Isolation", func(t *testing.T) {
		source := "claude"
		d := &GeminiDecision{
			AllTexts: map[string][]brain.Signed{
				source: {{Data: "original text", Signature: "sig_t"}},
			},
		}

		// 1. Get the clone
		exported := d.Texts()

		// 2. Mutate the clone by adding a new top-level map key
		exported["fake_source"] = []brain.Signed{{Data: "poison"}}

		// ASSERT: The original map does not contain the new key
		_, exists := d.AllTexts["fake_source"]
		assert.False(t, exists, "Map mutation leaked: Original map modified")
		assert.Len(t, d.AllTexts, 1)
	})
}

func TestGeminiDecision_AddSignFail(t *testing.T) {
	t.Run("Add: Bubble Up Internal Sign Error (line 191)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// 1. Setup a Decision with an existing Gemini entry
		d := &GeminiDecision{
			ChainOfThoughts: make(map[string][][]brain.Signed),
			AllPrompts:      make(map[string][]brain.Signed),
			AllTexts: map[string][]brain.Signed{
				"gemini": {{Data: "v1", Signature: "sig_1"}},
			},
		}

		source := "gemini"
		newCot := []brain.Signed{{Data: "thought", Signature: ""}}
		newText := brain.Signed{Data: "text", Signature: ""}

		// 2. Setup Mock to FAIL on the new signing request
		// The input will be B64(text) + sig_1 (the stitching)
		signErr := errors.New("cryptographic_module_failure")

		// We use .Maybe() for the existing block verification (if iteration hits it)
		mockSV.On("Verify", b64("v1")+"", "sig_1").Return(nil).Maybe()

		// THE TARGET: Force the recursive Sign call to fail
		mockSV.On("Sign", mock.Anything).Return("", signErr).Once()

		// 3. Execution
		err := d.Add(source, newCot, newText, mockSV)

		// 4. ASSERTIONS
		assert.ErrorIs(t, err, signErr, "Add failed to bubble the internal signing error")

		// Final Safety: Verify was NEVER called for the new data
		mockSV.AssertNotCalled(t, "Verify", b64("text")+"sig_1", mock.Anything)
	})
}

func TestGeminiDecision_Verify_DeepCoverage(t *testing.T) {
	mockSV := new(MockSignVerifier)

	t.Run("Verify: ChainOfThoughts Unsigned (line 134)", func(t *testing.T) {
		d := &GeminiDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"gemini": {{
					{Data: "valid thought", Signature: "sig_ok"},
					{Data: "unsigned thought", Signature: ""}, // THE TARGET
				}},
			},
		}

		// The first block might be verified if map order allows
		mockSV.On("Verify", b64("valid thought")+"", "sig_ok").Return(nil).Maybe()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})

	t.Run("Verify: ChainOfThoughts Deep Error (line 139)", func(t *testing.T) {
		d := &GeminiDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"gemini": {{
					{Data: "corrupt thought", Signature: "sig_bad"}, // THE TARGET
				}},
			},
		}

		deepErr := errors.New("chain_of_thought_tampered")
		mockSV.On("Verify", b64("corrupt thought")+"", "sig_bad").Return(deepErr).Once()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, deepErr)
	})

	t.Run("Verify: AllPrompts Happy vs Sad Path (lines 151-153)", func(t *testing.T) {
		d := &GeminiDecision{
			AllPrompts: map[string][]brain.Signed{
				"gemini": {
					{Data: "good prompt", Signature: "sig_p1"},
					{Data: "bad prompt", Signature: "sig_p2"}, // THE TARGET
				},
			},
		}

		// Happy path for p1
		mockSV.On("Verify", b64("good prompt")+"", "sig_p1").Return(nil).Maybe()
		// Sad path for p2
		promptErr := errors.New("prompt_signature_invalid")
		mockSV.On("Verify", b64("bad prompt")+"", "sig_p2").Return(promptErr).Once()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, promptErr)
	})

	t.Run("Verify: AllTexts Unsigned (line 162)", func(t *testing.T) {
		d := &GeminiDecision{
			AllTexts: map[string][]brain.Signed{
				"claude": {{Data: "text", Signature: ""}}, // THE TARGET
			},
		}

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})
}

func TestGeminiDecision_Sign_DeepCoverage(t *testing.T) {
	t.Run("Sign: ChainOfThoughts Peer Audit (line 51)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"claude": {{
					{Data: "peer thought", Signature: ""}, // THE TARGET: Unsigned peer data
				}},
			},
		}

		err := d.Sign(mockSV)

		// ASSERT: Gemini refuses to sign for Claude
		assert.ErrorIs(t, err, brain.ErrUnsigned)
		mockSV.AssertNotCalled(t, "Sign", mock.Anything)
	})

	t.Run("Sign: AllPrompts Happy/Unhappy Path (lines 65-68)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			AllPrompts: map[string][]brain.Signed{
				"gemini": {{Data: "my prompt", Signature: ""}},
			},
		}

		// Sad Path: Signer failure
		signErr := errors.New("kms_unreachable")
		mockSV.On("Sign", b64("my prompt")+"").Return("", signErr).Once()

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, signErr)

		// Happy Path: Reset and succeed
		mockSV = new(MockSignVerifier)
		mockSV.On("Sign", b64("my prompt")+"").Return("sig_p1", nil).Once()
		err = d.Sign(mockSV)
		assert.NoError(t, err)
		assert.Equal(t, "sig_p1", d.AllPrompts["gemini"][0].Signature)
	})

	t.Run("Sign: AllPrompts Peer Audit (line 73)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			AllPrompts: map[string][]brain.Signed{
				"claude": {{Data: "peer prompt", Signature: ""}}, // THE TARGET
			},
		}

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})

	t.Run("Sign: AllTexts Internal Failure (line 88)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			AllTexts: map[string][]brain.Signed{
				"gemini": {{Data: "final text", Signature: ""}},
			},
		}

		textSignErr := errors.New("entropy_failure")
		mockSV.On("Sign", b64("final text")+"").Return("", textSignErr).Once()

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, textSignErr)
	})

	t.Run("Sign: AllTexts Peer Audit (line 93)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &GeminiDecision{
			AllTexts: map[string][]brain.Signed{
				"claude": {{Data: "peer text", Signature: ""}}, // THE TARGET
			},
		}

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})
}
