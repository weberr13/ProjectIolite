package claude

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weberr13/ProjectIolite/brain"

	"github.com/anthropics/anthropic-sdk-go"
)

// Helper for cryptographic grounding
func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestClaudeResponse_Verify_DeepCoverage(t *testing.T) {
	mockSV := new(MockSignVerifier)

	t.Run("Verify: Nil Field Rejection (line 35)", func(t *testing.T) {
		// Target: Trigger ErrUnsigned when internal state is missing
		r := &ClaudeResponse{cot: nil, thought: nil}

		err := r.Verify(mockSV)

		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})

	t.Run("Verify: CoT Segment Failure (line 40)", func(t *testing.T) {
		// Target: Fail when one link in the chain is corrupt
		r := &ClaudeResponse{
			cot: []brain.Signed{
				{Data: "valid thought", Signature: "sig_ok"},
				{Data: "corrupt thought", Signature: "sig_bad"},
			},
			thought: &brain.Signed{Data: "final", Signature: "sig_f"},
		}

		// Success for first, failure for second
		mockSV.On("Verify", b64("valid thought")+"", "sig_ok").Return(nil).Once()

		cotErr := errors.New("cot_integrity_failure")
		mockSV.On("Verify", b64("corrupt thought")+"", "sig_bad").Return(cotErr).Once()

		err := r.Verify(mockSV)

		assert.ErrorIs(t, err, cotErr)
	})

	t.Run("Verify: Final Thought Failure (line 44)", func(t *testing.T) {
		// Target: The CoT is valid, but the final output is corrupt
		r := &ClaudeResponse{
			cot:     []brain.Signed{{Data: "valid", Signature: "sig_v"}},
			thought: &brain.Signed{Data: "tampered final", Signature: "sig_t"},
		}

		mockSV.On("Verify", b64("valid")+"", "sig_v").Return(nil).Once()

		finalErr := errors.New("thought_signature_mismatch")
		mockSV.On("Verify", b64("tampered final")+"", "sig_t").Return(finalErr).Once()

		err := r.Verify(mockSV)

		assert.ErrorIs(t, err, finalErr)
	})
}

func TestClaudeResponse_CandidatesToThoughts(t *testing.T) {
	t.Run("Chain: Recursive PrevSignature Stitching", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Sign", b64("Initial logic...")+"sig_p").Return("sig1", nil).Maybe()
		mockSV.On("Sign", b64("Refining strategy...")+"sig1").Return("sig2", nil).Maybe()
		rawJSON := `[
			{"type": "thinking", "thinking": "Initial logic..."},
			{"type": "thinking", "thinking": "Refining strategy..."}
		]`
		var content []anthropic.ContentBlockUnion
		json.Unmarshal([]byte(rawJSON), &content)
		resp := &anthropic.Message{Content: content}
		thoughts, err := candidatesToThoughts(mockSV, resp, brain.Signed{Signature: "sig_p"})

		assert.NoError(t, err)
		assert.Len(t, thoughts, 2)
		assert.Equal(t, "Initial logic...", thoughts[0].Data)
		assert.Equal(t, "Refining strategy...", thoughts[1].Data)
	})

	t.Run("Safety: Handle Empty Content", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		resp := &anthropic.Message{Content: []anthropic.ContentBlockUnion{}}
		thoughts, err := candidatesToThoughts(mockSV, resp, brain.Signed{Signature: "sig_p"})
		assert.NoError(t, err)
		assert.Empty(t, thoughts)
	})
}

func TestClaudeResponse_Sign(t *testing.T) {
	t.Run("Sign: Success Path with Prompt Grounding", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		prompt := brain.Signed{Data: "system prompt"}
		b, err := brain.NewBaseResponse(mockSV, "claude", prompt)
		assert.NoError(t, err)
		r := &ClaudeResponse{
			BaseResponse: b,
			cot: []brain.Signed{
				{Data: "thought", PrevSignature: ""},
			},
			thought: &brain.Signed{
				Data:          "final",
				PrevSignature: "sig_1",
			},
		}

		// 1. EXPECTATION: Signing the Prompt (The missing link!)
		mockSV.On("Sign", b64("system prompt")+"").Return("sig_p", nil).Once()

		// 2. EXPECTATION: Signing the CoT
		mockSV.On("Sign", b64("thought")+"").Return("sig_1", nil).Once()

		// 3. EXPECTATION: Signing the Final Thought
		mockSV.On("Sign", b64("final")+"sig_1").Return("sig_2", nil).Once()

		// 3. Execution
		err = r.Sign(mockSV)

		// 4. ASSERTIONS
		assert.NoError(t, err)
		assert.Equal(t, "sig_p", r.Prompt().Signature)
		assert.Equal(t, "sig_1", r.cot[0].Signature)
		assert.Equal(t, "sig_2", r.thought.Signature)

		mockSV.AssertExpectations(t)
	})

	t.Run("Sign: Fail on CoT Error", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		b, err := brain.NewBaseResponse(mockSV, "claude", brain.Signed{Data: "system"})
		assert.NoError(t, err)

		r := &ClaudeResponse{
			BaseResponse: b,
			cot:          []brain.Signed{{Data: "unlucky thought"}},
			thought:      &brain.Signed{Data: "placeholder"}, // PROTECT AGAINST NIL DEREF
		}

		signErr := errors.New("hsm_offline")

		// 1. Prompt signs successfully (assuming it's the first call)
		mockSV.On("Sign", b64("system")+"").Return("sig_p", nil).Once()

		// 2. CoT fails (The target branch)
		mockSV.On("Sign", b64("unlucky thought")+"").Return("", signErr).Once()

		err = r.Sign(mockSV)

		assert.ErrorIs(t, err, signErr)
		mockSV.AssertExpectations(t)
	})
}

func TestClaudeResponse_Sign_PromptFail(t *testing.T) {
	t.Run("Sign: Prompt Signing Failure (line 27)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		rawJSON := `{"type": "message", "content": [{"type": "text", "text": "final"}]}`
		var msg anthropic.Message
		json.Unmarshal([]byte(rawJSON), &msg)

		b, err := brain.NewBaseResponse(mockSV, "claude", brain.Signed{Data: "critical instructions", Signature: ""})
		assert.NoError(t, err)

		r := &ClaudeResponse{
			BaseResponse: b,
			resp:         &msg,
		}

		signErr := errors.New("prompt_locking_failed")
		mockSV.On("Sign", b64("critical instructions")+"").Return("", signErr).Once()

		err = r.Sign(mockSV)
		assert.ErrorIs(t, err, signErr)
		mockSV.AssertNotCalled(t, "Sign", b64("final")+"")
		mockSV.AssertExpectations(t)
	})
}
