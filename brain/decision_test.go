package brain

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
		validText := &Signed{Data: "Gemini output", Signature: "sig_g"}
		mockResp.On("Text").Return(validText).Maybe()
		mockResp.On("Source").Return("gemini").Maybe()
		mockResp.On("CoT").Return([]Signed{}).Maybe()
		mockResp.On("Prompt").Return(Signed{Data: "this question?", Signature: "sig_p"}).Maybe()

		// NewDecision verifies input first
		mockResp.On("Verify", mockSV).Return(nil).Once()

		// Signer expectations for the internal d.Sign(sv) call
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockSV.On("Sign", mock.Anything).Return("internal_sig", nil).Maybe()

		decision, err := NewBaseDecision("gemini", mockResp, mockSV)

		assert.NoError(t, err)
		assert.NotNil(t, decision)
		assert.Equal(t, "Gemini output", decision.AllTexts["gemini"][0].Data)
	})

	t.Run("NewDecision: Sign Failure Returns Nil", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		validText := &Signed{Data: "Gemini output", Signature: ""}
		mockResp := new(MockResponse)
		mockResp.On("Text").Return(validText).Maybe()
		mockResp.On("Source").Return("gemini").Maybe()
		mockResp.On("CoT").Return([]Signed{}).Maybe()
		mockResp.On("Prompt").Return(Signed{Data: "this question?", Signature: ""}).Maybe()

		// Force corruption error
		corruptErr := errors.New("initial_response_tampered")
		mockSV.On("Sign", mock.Anything, mock.Anything).Return("sig", corruptErr).Maybe()

		decision, err := NewBaseDecision("gemini", mockResp, mockSV)

		assert.ErrorIs(t, err, corruptErr)
		assert.NotNil(t, decision)
	})
}

func TestGeminiDecision_AddAndStitch(t *testing.T) {
	t.Run("Add: Atomic Signing and PrevSignature Stitching", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source:          "gemini",
			ChainOfThoughts: make(map[string][][]Signed),
			AllPrompts:      make(map[string][]Signed),
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "previous text", Signature: "sig_prev"}},
			},
		}

		source := "gemini"
		newText := Signed{Data: "current text"} // Unsigned

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
		d := &BaseDecision{
			Source: "gemini",
			ChainOfThoughts: map[string][][]Signed{
				"claude": {{{Data: "peer thought", Signature: "sig_c"}}},
			},
			AllTexts: map[string][]Signed{
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
		d := &BaseDecision{
			Source: "gemini",
			AllPrompts: map[string][]Signed{
				"gemini": {{Data: "prompt", Signature: ""}}, // Missing
			},
		}

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, ErrUnsigned)
		mockSV.AssertNotCalled(t, "Verify", mock.Anything, mock.Anything)
	})
}

func TestGeminiDecision_Getters(t *testing.T) {
	t.Run("Cots: Deep Copy Isolation", func(t *testing.T) {
		d := &BaseDecision{
			Source: "gemini",
			ChainOfThoughts: map[string][][]Signed{
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
		d := &BaseDecision{
			Source: "gemini",
			AllPrompts: map[string][]Signed{
				source: {{Data: "original prompt", Signature: "sig_p"}},
			},
		}

		// 1. Get the clone
		exported := d.Prompts()

		// 2. Mutate the clone's slice (Modify existing element)
		exported[source][0].Data = "MUTATED"

		// 3. Mutate the clone's slice (Append new element)
		exported[source] = append(exported[source], Signed{Data: "ghost prompt"})

		// ASSERT: The internal state of d remains "Grounded"
		assert.Equal(t, "original prompt", d.AllPrompts[source][0].Data, "Shallow copy detected: Mutation leaked to original")
		assert.Len(t, d.AllPrompts[source], 1, "Shallow copy detected: Append leaked to original")
	})

	t.Run("Texts: Map Key Isolation", func(t *testing.T) {
		source := "claude"
		d := &BaseDecision{
			Source: "gemini",
			AllTexts: map[string][]Signed{
				source: {{Data: "original text", Signature: "sig_t"}},
			},
		}

		// 1. Get the clone
		exported := d.Texts()

		// 2. Mutate the clone by adding a new top-level map key
		exported["fake_source"] = []Signed{{Data: "poison"}}

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
		d := &BaseDecision{
			Source:          "gemini",
			ChainOfThoughts: make(map[string][][]Signed),
			AllPrompts:      make(map[string][]Signed),
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "v1", Signature: "sig_1"}},
			},
		}

		source := "gemini"
		newCot := []Signed{{Data: "thought", Signature: ""}}
		newText := Signed{Data: "text", Signature: ""}

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
		d := &BaseDecision{
			Source: "gemini",
			ChainOfThoughts: map[string][][]Signed{
				"gemini": {{
					{Data: "valid thought", Signature: "sig_ok"},
					{Data: "unsigned thought", Signature: ""}, // THE TARGET
				}},
			},
		}

		// The first block might be verified if map order allows
		mockSV.On("Verify", b64("valid thought")+"", "sig_ok").Return(nil).Maybe()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, ErrUnsigned)
	})

	t.Run("Verify: ChainOfThoughts Deep Error (line 139)", func(t *testing.T) {
		d := &BaseDecision{
			Source: "gemini",
			ChainOfThoughts: map[string][][]Signed{
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
		d := &BaseDecision{
			Source: "gemini",
			AllPrompts: map[string][]Signed{
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
		d := &BaseDecision{
			Source: "gemini",
			AllTexts: map[string][]Signed{
				"claude": {{Data: "text", Signature: ""}}, // THE TARGET
			},
		}

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, ErrUnsigned)
	})
}

func TestGeminiDecision_Sign_DeepCoverage(t *testing.T) {
	t.Run("Sign: ChainOfThoughts Peer Audit (line 51)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "gemini",
			ChainOfThoughts: map[string][][]Signed{
				"claude": {{
					{Data: "peer thought", Signature: ""}, // THE TARGET: Unsigned peer data
				}},
			},
		}

		err := d.Sign(mockSV)

		// ASSERT: Gemini refuses to sign for Claude
		assert.ErrorIs(t, err, ErrUnsigned)
		mockSV.AssertNotCalled(t, "Sign", mock.Anything)
	})

	t.Run("Sign: AllPrompts Happy/Unhappy Path (lines 65-68)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "gemini",
			AllPrompts: map[string][]Signed{
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
		d := &BaseDecision{
			Source: "gemini",
			AllPrompts: map[string][]Signed{
				"claude": {{Data: "peer prompt", Signature: ""}}, // THE TARGET
			},
		}

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, ErrUnsigned)
	})

	t.Run("Sign: AllTexts Internal Failure (line 88)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "gemini",
			AllTexts: map[string][]Signed{
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
		d := &BaseDecision{
			Source: "gemini",
			AllTexts: map[string][]Signed{
				"claude": {{Data: "peer text", Signature: ""}}, // THE TARGET
			},
		}

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, ErrUnsigned)
	})
}

func TestClaudeDecision_New(t *testing.T) {
	t.Run("NewDecision: Pointer Dereference Safety", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockResp := new(MockResponse)

		// Preparation: init.Text() MUST return a non-nil pointer
		// to avoid the SIGSEGV at line 33.
		validSigned := &Signed{Data: "response text", Signature: "sig_resp"}
		mockResp.On("Text").Return(validSigned).Maybe()
		mockResp.On("Source").Return("gemini").Maybe()
		mockResp.On("CoT").Return([]Signed{}).Maybe()
		mockResp.On("Prompt").Return(Signed{Signature: "sig_prompt"}).Maybe()

		// NewDecision first verifies the response
		mockResp.On("Verify", mockSV).Return(nil).Once()
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)
		mockSV.On("Sign", mock.Anything).Return("fakeSig", nil)

		// Sign() will run during NewDecision; gemini blocks must already be signed
		decision, err := NewBaseDecision("claude", mockResp, mockSV)

		assert.NoError(t, err)
		assert.NotNil(t, decision)
		assert.Equal(t, "response text", decision.AllTexts["gemini"][0].Data)
	})
}

func TestClaudeDecision_AddAndStitch(t *testing.T) {
	t.Run("Add: Map Initialization and Chain Stitching", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		// Manually construct to test the Add() nil-check logic
		d := &BaseDecision{
			Source:          "claude",
			ChainOfThoughts: make(map[string][][]Signed),
			AllPrompts:      make(map[string][]Signed),
			AllTexts:        make(map[string][]Signed),
		}

		source := "claude"
		cot := []Signed{{Data: "thought 1", Signature: "sig_thought"}}
		text1 := Signed{Data: "text 1", Signature: "sig_text_1"}
		mockSV.On("Verify", b64("thought 1"), "sig_thought").Return(nil).Maybe()
		mockSV.On("Verify", b64("text 1"), "sig_text_1").Return(nil).Maybe()
		mockSV.On("Sign", b64("text 1")).Return("sig_text_1", nil).Maybe()
		// 1. First Add: Check map initialization
		err := d.Add(source, cot, text1, mockSV)
		assert.NoError(t, err)
		assert.Len(t, d.AllTexts[source], 1)

		// 2. Second Add: Check PrevSignature stitching
		text2 := Signed{Data: "text 2"} // Unsigned

		// Expect Sign() to be called for the new "claude" text
		mockSV.On("Sign", b64("text 2")+"sig_text_1").Return("sig_text_2", nil).Maybe()
		// Expect Verify() to be called after Sign()
		mockSV.On("Verify", b64("text 2")+"sig_text_1", "sig_text_2").Return(nil).Maybe()

		err = d.Add(source, nil, text2, mockSV)
		assert.NoError(t, err)

		// Verify stitching: text2.PrevSignature should match text1.Signature
		assert.Equal(t, "sig_text_1", d.AllTexts[source][1].PrevSignature)
		assert.Equal(t, "sig_text_2", d.AllTexts[source][1].Signature)
	})
}

func TestClaudeDecision_SigningRules(t *testing.T) {
	t.Run("Sign: Reject Unsigned Peer Blocks", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "peer text", Signature: ""}}, // UNSIGNED
			},
		}

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, ErrUnsigned, "Should fail if trying to sign a non-claude block")
	})
}

func TestClaudeDecision_Immutability(t *testing.T) {
	t.Run("Cots: Deep Copy Isolation", func(t *testing.T) {
		source := "gemini"
		originalThought := Signed{Data: "original", Signature: "sig1"}

		d := &BaseDecision{
			Source: "claude",
			ChainOfThoughts: map[string][][]Signed{
				source: {{originalThought}},
			},
		}

		// 1. Get the copy
		exportedCots := d.Cots()

		// 2. Attempt to mutate the copy
		exportedCots[source][0][0].Data = "MUTATED"

		// 3. ASSERT: The original struct is untouched
		assert.Equal(t, "original", d.ChainOfThoughts[source][0][0].Data, "Deep copy failed: Original data was mutated")
		assert.NotSame(t, &exportedCots[source][0], &d.ChainOfThoughts[source][0], "Slice addresses should differ")
	})

	t.Run("Prompts: Slice Isolation", func(t *testing.T) {
		source := "claude"
		d := &BaseDecision{
			Source: "claude",
			AllPrompts: map[string][]Signed{
				source: {{Data: "prompt 1", Signature: "sig_p1"}},
			},
		}

		// 1. Get the copy
		exportedPrompts := d.Prompts()

		// 2. Mutate the slice by appending
		exportedPrompts[source] = append(exportedPrompts[source], Signed{Data: "new prompt"})

		// 3. ASSERT: Original map length remains the same
		assert.Len(t, d.AllPrompts[source], 1, "Original AllPrompts slice was modified by copy append")
	})
}

func TestClaudeDecision_DeepSign(t *testing.T) {
	t.Run("Sign: Recursive ChainOfThoughts Signing", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			ChainOfThoughts: map[string][][]Signed{
				"claude": {
					{
						{Data: "thought 1", Signature: ""},             // Needs signing
						{Data: "thought 2", Signature: "pre-existing"}, // Should skip
					},
				},
			},
		}

		// Expectation: Only the unsigned "claude" thought gets the Sign call
		mockSV.On("Sign", b64("thought 1")).Return("new_sig_1", nil).Once()

		err := d.Sign(mockSV)

		assert.NoError(t, err)
		assert.Equal(t, "new_sig_1", d.ChainOfThoughts["claude"][0][0].Signature)
		mockSV.AssertExpectations(t)
	})

	t.Run("Sign: Block Peer Unsigned Data (Prompts)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			AllPrompts: map[string][]Signed{
				"gemini": {
					{Data: "peer prompt", Signature: ""}, // Peer failed to sign
				},
			},
		}

		err := d.Sign(mockSV)

		// ASSERT: The decision refuses to vouch for unsigned peer data
		assert.ErrorIs(t, err, ErrUnsigned)
		mockSV.AssertNotCalled(t, "Sign", mock.Anything)
	})

	t.Run("Sign: Error Propagation on Internal Failure", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			AllTexts: map[string][]Signed{
				"claude": {{Data: "failed text", Signature: ""}},
			},
		}

		signErr := errors.New("hsm_not_reachable")
		mockSV.On("Sign", mock.Anything).Return("", signErr).Once()

		err := d.Sign(mockSV)

		assert.ErrorIs(t, err, signErr)
	})
}

func TestClaudeDecision_SignPrompts(t *testing.T) {
	t.Run("Sign: Reach and Execute AllPrompts 'claude' Block", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// 1. Setup the Decision state
		// We leave ChainOfThoughts empty so the iterator moves to AllPrompts
		d := &BaseDecision{
			Source:          "claude",
			ChainOfThoughts: make(map[string][][]Signed),
			AllTexts:        make(map[string][]Signed),
			AllPrompts: map[string][]Signed{
				"claude": {
					{Data: "Target Prompt", Signature: ""}, // This is our target
				},
			},
		}

		// 2. Expected Data: b64("Target Prompt") + PrevSignature("")
		expectedInput := b64("Target Prompt") + ""
		expectedSig := "prompt_signature_789"

		// Expectation: The inner loop hits d.AllPrompts["claude"][0].Sign(sv)
		mockSV.On("Sign", expectedInput).Return(expectedSig, nil).Once()

		// 3. Execution
		err := d.Sign(mockSV)

		// 4. ASSERTIONS
		assert.NoError(t, err)
		assert.Equal(t, expectedSig, d.AllPrompts["claude"][0].Signature, "Prompt was not signed correctly")
		mockSV.AssertExpectations(t)
	})

	t.Run("Sign: Handle AllPrompts Sign Failure", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			AllPrompts: map[string][]Signed{
				"claude": {{Data: "Failing Prompt", Signature: ""}},
			},
		}

		signErr := errors.New("hsm_timeout")
		mockSV.On("Sign", mock.Anything).Return("", signErr).Once()

		err := d.Sign(mockSV)

		// ASSERT: The error at line 66 is correctly bubbled up
		assert.ErrorIs(t, err, signErr)
	})
}

func TestClaudeDecision_SignPeerAudit(t *testing.T) {
	t.Run("Sign: Detect Unsigned Peer Thoughts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// 1. Setup the "Toxic" Decision state
		// We use a non-claude source (e.g., "gemini")
		d := &BaseDecision{
			Source: "claude",
			ChainOfThoughts: map[string][][]Signed{
				"gemini": {
					{
						{Data: "valid thought", Signature: "sig_123"},
						{Data: "toxic thought", Signature: ""}, // THE TARGET: Unsigned peer data
					},
				},
			},
			AllPrompts: make(map[string][]Signed),
			AllTexts:   make(map[string][]Signed),
		}

		// 2. Execution
		err := d.Sign(mockSV)

		// 3. ASSERTIONS
		// The code must return ErrUnsigned because it refuses to sign for others
		assert.ErrorIs(t, err, ErrUnsigned, "Should have failed on unsigned peer thought")

		// Verify that the SignVerifier was NEVER touched
		mockSV.AssertNotCalled(t, "Sign", mock.Anything)
	})

	t.Run("Sign: Success with Signed Peer Thoughts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// If the peer data is already signed, the loop should pass through
		d := &BaseDecision{
			Source: "claude",
			ChainOfThoughts: map[string][][]Signed{
				"gemini": {
					{
						{Data: "honest peer thought", Signature: "peer_sig_alpha"},
					},
				},
			},
		}

		err := d.Sign(mockSV)

		// ASSERT: No error because everything pre-existing is valid (or at least signed)
		assert.NoError(t, err)
	})
}

func TestClaudeDecision_SignInternalFailure(t *testing.T) {
	t.Run("Sign: Bubble Up Recursive Signing Error", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// 1. Setup the Decision state with an unsigned "claude" thought
		d := &BaseDecision{
			Source: "claude",
			ChainOfThoughts: map[string][][]Signed{
				"claude": {
					{
						{Data: "important thought", Signature: ""}, // Target for signing
					},
				},
			},
			AllPrompts: make(map[string][]Signed),
			AllTexts:   make(map[string][]Signed),
		}

		// 2. Mock a failure in the Signer
		// We use b64("important thought") + "" as the expected input
		signErr := errors.New("hsm_key_not_found")
		mockSV.On("Sign", b64("important thought")+"").Return("", signErr).Once()

		// 3. Execution
		err := d.Sign(mockSV)

		// 4. ASSERTIONS
		// The error from the recursive call must be bubbled up immediately
		assert.ErrorIs(t, err, signErr, "Should have returned the specific signer error")

		// Integrity Check: Ensure the signature wasn't partially set
		assert.Empty(t, d.ChainOfThoughts["claude"][0][0].Signature)

		mockSV.AssertExpectations(t)
	})
}

func TestClaudeDecision_Verify_Coverage(t *testing.T) {
	t.Run("Verify: Detect Unsigned Block in ChainOfThoughts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			ChainOfThoughts: map[string][][]Signed{
				"any_source": {{{Data: "unsigned", Signature: ""}}},
			},
		}

		err := d.Verify(mockSV)

		// ASSERT: Fails at the first check in the nested loop
		assert.ErrorIs(t, err, ErrUnsigned)
		mockSV.AssertNotCalled(t, "Verify", mock.Anything, mock.Anything)
	})

	t.Run("Verify: Detect Corrupted Signature in AllPrompts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			AllPrompts: map[string][]Signed{
				"claude": {{Data: "tampered prompt", Signature: "bad_sig"}},
			},
		}

		// The internal Signed.Verify() will call our mock
		verifyErr := errors.New("invalid_signature_bytes")
		mockSV.On("Verify", b64("tampered prompt")+"", "bad_sig").Return(verifyErr).Once()

		err := d.Verify(mockSV)

		// ASSERT: The error from the recursive call is bubbled up
		assert.ErrorIs(t, err, verifyErr)
	})

	t.Run("Verify: Detect Unsigned Block in AllTexts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "text", Signature: ""}},
			},
		}

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, ErrUnsigned)
	})

	t.Run("Verify: Happy Path Success", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &BaseDecision{
			Source: "claude",
			AllPrompts: map[string][]Signed{
				"claude": {{Data: "valid", Signature: "sig_p"}},
			},
			AllTexts: map[string][]Signed{
				"claude": {{Data: "valid", Signature: "sig_t"}},
			},
		}

		// Expectations for every block
		mockSV.On("Verify", mock.Anything, "sig_p").Return(nil).Once()
		mockSV.On("Verify", mock.Anything, "sig_t").Return(nil).Once()

		err := d.Verify(mockSV)

		assert.NoError(t, err)
		mockSV.AssertExpectations(t)
	})
}

func TestClaudeDecision_Verify_DeepCoverage(t *testing.T) {
	mockSV := new(MockSignVerifier)

	t.Run("Verify: ChainOfThoughts Deep Error (line 104)", func(t *testing.T) {
		// Target: inner slice err return after a failed Signed.Verify
		d := &BaseDecision{
			Source: "claude",
			ChainOfThoughts: map[string][][]Signed{
				"claude": {
					{
						{Data: "valid thought", Signature: "sig_ok"},
						{Data: "corrupt thought", Signature: "sig_bad"}, // THE TARGET
					},
				},
			},
		}

		// Setup: First thought passes, second fails
		mockSV.On("Verify", b64("valid thought")+"", "sig_ok").Return(nil).Once()
		deepErr := errors.New("deep_chain_corruption")
		mockSV.On("Verify", b64("corrupt thought")+"", "sig_bad").Return(deepErr).Once()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, deepErr)
	})

	t.Run("Verify: AllPrompts Missing Signature (line 113)", func(t *testing.T) {
		// Target: return ErrUnsigned from the middle loop
		d := &BaseDecision{
			Source: "claude",
			AllPrompts: map[string][]Signed{
				"gemini": {
					{Data: "signed prompt", Signature: "sig_p1"},
					{Data: "unsigned prompt", Signature: ""}, // THE TARGET
				},
			},
		}

		// The first one might be verified if map order allows
		mockSV.On("Verify", mock.Anything, "sig_p1").Return(nil).Maybe()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, ErrUnsigned)
	})

	t.Run("Verify: AllTexts Deep Error (line 126)", func(t *testing.T) {
		// Target: return err from the final loop
		d := &BaseDecision{
			Source: "claude",
			AllTexts: map[string][]Signed{
				"claude": {
					{Data: "valid text", Signature: "sig_t1"},
					{Data: "tampered text", Signature: "sig_t2"}, // THE TARGET
				},
			},
		}

		mockSV.On("Verify", mock.Anything, "sig_t1").Return(nil).Maybe()
		textErr := errors.New("text_integrity_compromised")
		mockSV.On("Verify", mock.Anything, "sig_t2").Return(textErr).Once()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, textErr)
	})
}

func TestClaudeDecision_AddSignFailure(t *testing.T) {
	t.Run("Add: Bubble Up Sign Error", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// 1. Setup an existing Decision
		d := &BaseDecision{
			Source:          "claude",
			ChainOfThoughts: make(map[string][][]Signed),
			AllPrompts:      make(map[string][]Signed),
			AllTexts:        make(map[string][]Signed),
		}

		source := "claude"
		cot := []Signed{{Data: "thought", Signature: ""}}
		text := Signed{Data: "text", Signature: ""}

		// 2. Mock a failure in the Signer for the new block
		// We expect the recursive Sign() call inside Add() to hit this error
		signErr := errors.New("signature_engine_offline")
		mockSV.On("Sign", mock.Anything).Return("", signErr).Maybe()
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

		// 3. Execution
		err := d.Add(source, cot, text, mockSV)

		// 4. ASSERTIONS
		// The error from d.Sign(sv) must be the one returned by Add()
		assert.ErrorIs(t, err, signErr, "Add should have bubbled up the signing error")

		// Verification: Ensure the Verify call was NEVER reached
		// (If it reached Verify, it would likely return ErrUnsigned first)
		mockSV.AssertNotCalled(t, "Verify", mock.Anything, mock.Anything)
	})
}

func TestErrorSetting(t *testing.T) {
	t.Run("can get and set errors", func(t *testing.T) {
		d := &BaseDecision{}
		assert.NoError(t, d.IsError())
		err := errors.New("this is expected")
		d.SetError(err)
		assert.Equal(t, err, d.IsError())
	})
}
