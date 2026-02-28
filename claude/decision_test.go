package claude

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
)

func TestClaudeDecision_New(t *testing.T) {
	t.Run("NewDecision: Pointer Dereference Safety", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockResp := new(MockResponse)

		// Preparation: init.Text() MUST return a non-nil pointer
		// to avoid the SIGSEGV at line 33.
		validSigned := &brain.Signed{Data: "response text", Signature: "sig_resp"}
		mockResp.On("Text").Return(validSigned).Maybe()
		mockResp.On("Source").Return("gemini").Maybe()
		mockResp.On("CoT").Return([]brain.Signed{}).Maybe()
		mockResp.On("Prompt").Return(brain.Signed{Signature: "sig_prompt"}).Maybe()

		// NewDecision first verifies the response
		mockResp.On("Verify", mockSV).Return(nil).Once()
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)
		mockSV.On("Sign", mock.Anything).Return("fakeSig", nil)

		// Sign() will run during NewDecision; gemini blocks must already be signed
		decision, err := NewDecision(mockResp, mockSV)

		assert.NoError(t, err)
		assert.NotNil(t, decision)
		assert.Equal(t, "response text", decision.AllTexts["gemini"][0].Data)
	})

	t.Run("NewDecision: Integrity Failure", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockResp := new(MockResponse)
		mockResp.On("Source").Return("gemini").Maybe()
		mockResp.On("Text").Return(&brain.Signed{Data: "text", Signature: "sig"}).Maybe()
		mockResp.On("Prompt").Return(brain.Signed{Signature: "sig"}).Maybe()
		corruptErr := errors.New("signature_mismatch")
		mockResp.On("Verify", mockSV).Return(corruptErr).Once()

		decision, err := NewDecision(mockResp, mockSV)

		assert.ErrorIs(t, err, corruptErr)
		assert.Nil(t, decision, "NewDecision MUST return nil if input is corrupt")
	})
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestClaudeDecision_AddAndStitch(t *testing.T) {
	t.Run("Add: Map Initialization and Chain Stitching", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		// Manually construct to test the Add() nil-check logic
		d := &ClaudeDecision{
			ChainOfThoughts: make(map[string][][]brain.Signed),
			AllPrompts:      make(map[string][]brain.Signed),
			AllTexts:        make(map[string][]brain.Signed),
		}

		source := "claude"
		cot := []brain.Signed{{Data: "thought 1", Signature: "sig_thought"}}
		text1 := brain.Signed{Data: "text 1", Signature: "sig_text_1"}
		mockSV.On("Verify", b64("thought 1"), "sig_thought").Return(nil).Maybe()
		mockSV.On("Verify", b64("text 1"), "sig_text_1").Return(nil).Maybe()
		mockSV.On("Sign", b64("text 1")).Return("sig_text_1", nil).Maybe()
		// 1. First Add: Check map initialization
		err := d.Add(source, cot, text1, mockSV)
		assert.NoError(t, err)
		assert.Len(t, d.AllTexts[source], 1)

		// 2. Second Add: Check PrevSignature stitching
		text2 := brain.Signed{Data: "text 2"} // Unsigned

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
		d := &ClaudeDecision{
			AllTexts: map[string][]brain.Signed{
				"gemini": {{Data: "peer text", Signature: ""}}, // UNSIGNED
			},
		}

		err := d.Sign(mockSV)
		assert.ErrorIs(t, err, brain.ErrUnsigned, "Should fail if trying to sign a non-claude block")
	})
}

func TestClaudeDecision_Immutability(t *testing.T) {
	t.Run("Cots: Deep Copy Isolation", func(t *testing.T) {
		source := "gemini"
		originalThought := brain.Signed{Data: "original", Signature: "sig1"}

		d := &ClaudeDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
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
		d := &ClaudeDecision{
			AllPrompts: map[string][]brain.Signed{
				source: {{Data: "prompt 1", Signature: "sig_p1"}},
			},
		}

		// 1. Get the copy
		exportedPrompts := d.Prompts()

		// 2. Mutate the slice by appending
		exportedPrompts[source] = append(exportedPrompts[source], brain.Signed{Data: "new prompt"})

		// 3. ASSERT: Original map length remains the same
		assert.Len(t, d.AllPrompts[source], 1, "Original AllPrompts slice was modified by copy append")
	})
}

func TestClaudeDecision_DeepSign(t *testing.T) {
	t.Run("Sign: Recursive ChainOfThoughts Signing", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &ClaudeDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
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
		d := &ClaudeDecision{
			AllPrompts: map[string][]brain.Signed{
				"gemini": {
					{Data: "peer prompt", Signature: ""}, // Peer failed to sign
				},
			},
		}

		err := d.Sign(mockSV)

		// ASSERT: The decision refuses to vouch for unsigned peer data
		assert.ErrorIs(t, err, brain.ErrUnsigned)
		mockSV.AssertNotCalled(t, "Sign", mock.Anything)
	})

	t.Run("Sign: Error Propagation on Internal Failure", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &ClaudeDecision{
			AllTexts: map[string][]brain.Signed{
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
		d := &ClaudeDecision{
			ChainOfThoughts: make(map[string][][]brain.Signed),
			AllTexts:        make(map[string][]brain.Signed),
			AllPrompts: map[string][]brain.Signed{
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
		d := &ClaudeDecision{
			AllPrompts: map[string][]brain.Signed{
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
		d := &ClaudeDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"gemini": {
					{
						{Data: "valid thought", Signature: "sig_123"},
						{Data: "toxic thought", Signature: ""}, // THE TARGET: Unsigned peer data
					},
				},
			},
			AllPrompts: make(map[string][]brain.Signed),
			AllTexts:   make(map[string][]brain.Signed),
		}

		// 2. Execution
		err := d.Sign(mockSV)

		// 3. ASSERTIONS
		// The code must return ErrUnsigned because it refuses to sign for others
		assert.ErrorIs(t, err, brain.ErrUnsigned, "Should have failed on unsigned peer thought")

		// Verify that the SignVerifier was NEVER touched
		mockSV.AssertNotCalled(t, "Sign", mock.Anything)
	})

	t.Run("Sign: Success with Signed Peer Thoughts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// If the peer data is already signed, the loop should pass through
		d := &ClaudeDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
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
		d := &ClaudeDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"claude": {
					{
						{Data: "important thought", Signature: ""}, // Target for signing
					},
				},
			},
			AllPrompts: make(map[string][]brain.Signed),
			AllTexts:   make(map[string][]brain.Signed),
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
		d := &ClaudeDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
				"any_source": {{{Data: "unsigned", Signature: ""}}},
			},
		}

		err := d.Verify(mockSV)

		// ASSERT: Fails at the first check in the nested loop
		assert.ErrorIs(t, err, brain.ErrUnsigned)
		mockSV.AssertNotCalled(t, "Verify", mock.Anything, mock.Anything)
	})

	t.Run("Verify: Detect Corrupted Signature in AllPrompts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &ClaudeDecision{
			AllPrompts: map[string][]brain.Signed{
				"claude": {{Data: "tampered prompt", Signature: "bad_sig"}},
			},
		}

		// The internal brain.Signed.Verify() will call our mock
		verifyErr := errors.New("invalid_signature_bytes")
		mockSV.On("Verify", b64("tampered prompt")+"", "bad_sig").Return(verifyErr).Once()

		err := d.Verify(mockSV)

		// ASSERT: The error from the recursive call is bubbled up
		assert.ErrorIs(t, err, verifyErr)
	})

	t.Run("Verify: Detect Unsigned Block in AllTexts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &ClaudeDecision{
			AllTexts: map[string][]brain.Signed{
				"gemini": {{Data: "text", Signature: ""}},
			},
		}

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})

	t.Run("Verify: Happy Path Success", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		d := &ClaudeDecision{
			AllPrompts: map[string][]brain.Signed{
				"claude": {{Data: "valid", Signature: "sig_p"}},
			},
			AllTexts: map[string][]brain.Signed{
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
		// Target: inner slice err return after a failed brain.Signed.Verify
		d := &ClaudeDecision{
			ChainOfThoughts: map[string][][]brain.Signed{
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
		// Target: return brain.ErrUnsigned from the middle loop
		d := &ClaudeDecision{
			AllPrompts: map[string][]brain.Signed{
				"gemini": {
					{Data: "signed prompt", Signature: "sig_p1"},
					{Data: "unsigned prompt", Signature: ""}, // THE TARGET
				},
			},
		}

		// The first one might be verified if map order allows
		mockSV.On("Verify", mock.Anything, "sig_p1").Return(nil).Maybe()

		err := d.Verify(mockSV)

		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})

	t.Run("Verify: AllTexts Deep Error (line 126)", func(t *testing.T) {
		// Target: return err from the final loop
		d := &ClaudeDecision{
			AllTexts: map[string][]brain.Signed{
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
	t.Run("Add: Bubble Up Sign Error (line 155)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// 1. Setup an existing Decision
		d := &ClaudeDecision{
			ChainOfThoughts: make(map[string][][]brain.Signed),
			AllPrompts:      make(map[string][]brain.Signed),
			AllTexts:        make(map[string][]brain.Signed),
		}

		source := "claude"
		cot := []brain.Signed{{Data: "thought", Signature: ""}}
		text := brain.Signed{Data: "text", Signature: ""}

		// 2. Mock a failure in the Signer for the new block
		// We expect the recursive Sign() call inside Add() to hit this error
		signErr := errors.New("signature_engine_offline")
		mockSV.On("Sign", mock.Anything).Return("", signErr).Once()

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
