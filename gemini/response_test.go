package gemini

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
	"google.golang.org/genai"
)

func TestGeminiResponse_CandidatesToThoughts(t *testing.T) {
	t.Run("Chain: Gemini Part-based Thought Extraction", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Sign", mock.Anything).Return("sig_1", nil).Maybe()

		// 1. Construct a mock Gemini response
		// Note: Gemini uses boolean flags on Parts to indicate 'Thought'
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Parts: []*genai.Part{
							{Text: "Initial Gemini logic...", Thought: true},
							{Text: "This is just output text", Thought: false},
							{Text: "Refining Gemini strategy...", Thought: true},
						},
					},
				},
			},
		}

		// 2. Execution
		thoughts, err := candidatesToThoughts(mockSV, resp, brain.Signed{Signature: "sig_p"})
		assert.NoError(t, err)

		// 3. ASSERTIONS
		assert.Len(t, thoughts, 2)
		assert.Equal(t, "Initial Gemini logic...", thoughts[0].Data)
		// Verification of the NextUnsigned stitching
		assert.Equal(t, "cot", thoughts[1].Namespace)
		assert.Equal(t, "Refining Gemini strategy...", thoughts[1].Data)
	})
}

func TestGeminiResponse_SignAndVerify(t *testing.T) {
	mockSV := new(MockSignVerifier)

	t.Run("Sign: Full Manifest Grounding", func(t *testing.T) {
		b := brain.NewBaseResponse("gemini", brain.Signed{Data: "gemini prompt"})
		r := &GeminiResponse{
			BaseResponse: b,
			resp:         &genai.GenerateContentResponse{}, // Empty but non-nil
			model:        "gemini-2.0-flash",
		}

		// Expectations for Prompt, then Thought (Lazy loaded from Text())
		mockSV.On("Sign", b64("gemini prompt")+"").Return("sig_p", nil).Once()
		// GeminiResponse.Text() uses r.resp.Text(), which defaults to empty if resp is empty
		mockSV.On("Sign", b64("")+"sig_p").Return("sig_t", nil).Once()

		err := r.Sign(mockSV)

		assert.NoError(t, err)
		assert.Equal(t, "sig_p", r.Prompt().Signature)
		assert.Equal(t, "sig_t", r.thought.Signature)
	})

	t.Run("Verify: Short-circuit on Unsigned", func(t *testing.T) {
		r := &GeminiResponse{cot: nil, thought: nil}
		err := r.Verify(mockSV)
		assert.ErrorIs(t, err, brain.ErrUnsigned)
	})
}

func TestGeminiResponse_Identity(t *testing.T) {
	t.Run("Prompt: Pass-through Integrity", func(t *testing.T) {
		// Even if everything else is nil, the prompt must be a value type
		expected := brain.Signed{
			Data:      "Analyze the following logs...",
			Namespace: "prompt",
			Signature: "sig_123",
		}
		b := brain.NewBaseResponse("gemini", expected)
		r := &GeminiResponse{BaseResponse: b}

		got := r.Prompt()

		assert.Equal(t, expected.Data, got.Data)
		assert.Equal(t, expected.Signature, got.Signature)
		assert.Equal(t, "prompt", got.Namespace)
	})
}

func TestGeminiResponse_CandidatesToThoughts_Pathological(t *testing.T) {
	t.Run("Panic: Nil Candidate and Nil Part Dereference", func(t *testing.T) {
		// Construct a response with nil pointers in the slices
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				nil, // PATHOLOGY 1: Nil candidate entry
				{
					Content: &genai.Content{
						Parts: []*genai.Part{
							nil, // PATHOLOGY 2: Nil part entry
							{Text: "Valid thought", Thought: true},
						},
					},
				},
				{
					Content: nil, // PATHOLOGY 3: Candidate with nil Content
				},
			},
		}

		// ASSERT: This will likely panic on 'c.Content.Parts' or 'p.Thought'
		assert.NotPanics(t, func() {
			sv := new(MockSignVerifier)
			sv.On("Sign", mock.Anything).Return("sig_1", nil).Maybe()

			_, err := candidatesToThoughts(sv, resp, brain.Signed{Signature: "sig_p"})
			if err != nil {
				panic(err)
			}
		}, "candidatesToThoughts should be resilient to nil pointers in slices")
	})
}

func TestGeminiResponse_CandidatesToThoughts_Boundaries(t *testing.T) {
	t.Run("Safety: Global Nil Response", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Sign", mock.Anything).Return("sig_1", nil).Maybe()
		// Target: Trigger 'if resp == nil'
		var nilResp *genai.GenerateContentResponse
		thoughts, err := candidatesToThoughts(mockSV, nilResp, brain.Signed{Signature: "sig_p"})
		assert.NoError(t, err)
		assert.Empty(t, thoughts, "Should return empty slice for nil response")
	})

	t.Run("Safety: Sparse and Nil Candidates", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		// Target: Trigger 'if c == nil || c.Content == nil'
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				nil,            // Trigger: c == nil
				{Content: nil}, // Trigger: c.Content == nil
				{Content: &genai.Content{Parts: []*genai.Part{}}}, // Empty parts
			},
		}

		thoughts, err := candidatesToThoughts(mockSV, resp, brain.Signed{Signature: "sig_p"})
		assert.NoError(t, err)
		assert.Empty(t, thoughts)
	})

	t.Run("Safety: Nil Parts and Non-Thoughts", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Sign", mock.Anything).Return("sig_1", nil).Maybe()

		// Target: Trigger 'if p == nil || !p.Thought'
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Parts: []*genai.Part{
							nil,                                     // Trigger: p == nil
							{Text: "Not a thought", Thought: false}, // Trigger: !p.Thought
							{Text: "Actual thought", Thought: true}, // Valid path
						},
					},
				},
			},
		}

		thoughts, err := candidatesToThoughts(mockSV, resp, brain.Signed{Signature: "sig_p"})
		assert.NoError(t, err)
		assert.Len(t, thoughts, 1)
		assert.Equal(t, "Actual thought", thoughts[0].Data)
	})
}

func TestGeminiResponse_Cryptography(t *testing.T) {
	mockSV := new(MockSignVerifier)

	t.Run("Sign: Prompt Failure", func(t *testing.T) {
		// Target: Force 'err != nil' on prompt signing
		b := brain.NewBaseResponse("gemini", brain.Signed{Data: "system prompt"})
		r := &GeminiResponse{
			BaseResponse: b,
			resp:         &genai.GenerateContentResponse{}, // satisfy lazy init
		}

		signErr := errors.New("hsm_key_locked")
		mockSV.On("Sign", b64("system prompt")+"").Return("", signErr).Once()

		err := r.Sign(mockSV)
		assert.ErrorIs(t, err, signErr)
	})

	t.Run("Sign: CoT Chain Failure", func(t *testing.T) {
		b := brain.NewBaseResponse("gemini", brain.Signed{Data: "p", Signature: "sig_p"})
		r := &GeminiResponse{
			BaseResponse: b,
			cot: []brain.Signed{
				{Data: "step 1"},
				{Data: "step 2"},
			},
			thought: &brain.Signed{Data: "final"},
		}

		// First thought signs successfully
		mockSV.On("Sign", b64("step 1")+"sig_p").Return("sig_1", nil).Once()
		// Second thought fails
		cotErr := errors.New("signature_buffer_full")
		mockSV.On("Sign", b64("step 2")+"sig_1").Return("", cotErr).Once()

		err := r.Sign(mockSV)
		assert.ErrorIs(t, err, cotErr)
	})

	t.Run("Verify: CoT Tamper Detection", func(t *testing.T) {
		// Target: Force 'err != nil' in the verification loop
		r := &GeminiResponse{
			cot: []brain.Signed{
				{Data: "valid", Signature: "v_sig"},
				{Data: "tampered", Signature: "t_sig"},
			},
			thought: &brain.Signed{Data: "ignored"},
		}

		mockSV.On("Verify", b64("valid")+"", "v_sig").Return(nil).Once()

		verifyErr := errors.New("invalid_signature")
		mockSV.On("Verify", b64("tampered")+"", "t_sig").Return(verifyErr).Once()

		err := r.Verify(mockSV)
		assert.ErrorIs(t, err, verifyErr)
	})

	t.Run("Sign: CoT Chain Success (Internal Stitching Only)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		b := brain.NewBaseResponse("gemini", brain.Signed{Data: "p", Signature: "sig_p"})
		r := &GeminiResponse{
			BaseResponse: b,
			cot: []brain.Signed{
				{Data: "step 1"},
				{Data: "step 2"},
			},
			thought: &brain.Signed{Data: "final"},
		}

		// 1. Prompt signs independently (or is already signed)
		// 2. First thought: lastSig is "", so no salt
		mockSV.On("Sign", b64("step 1")+"sig_p").Return("sig_1", nil).Once()

		// 3. Second thought: lastSig is "sig_1", so it chains!
		mockSV.On("Sign", b64("step 2")+"sig_1").Return("sig_2", nil).Once()

		// 4. Final thought: lastSig is "sig_2", so it chains to the end of the CoT
		mockSV.On("Sign", b64("final")+"sig_2").Return("sig_f", nil).Once()

		err := r.Sign(mockSV)

		assert.NoError(t, err)
		assert.Equal(t, "sig_1", r.cot[0].Signature)
		assert.Equal(t, "sig_2", r.cot[1].Signature)
		assert.Equal(t, "sig_f", r.thought.Signature)

		mockSV.AssertExpectations(t)
	})
}

func TestGeminiResponse_Describe(t *testing.T) {
	t.Run("Describe: Verify Manifest Formatting and B64 Encoding", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockSV.On("ExportPublicKey").Return("iolite_pk_test_001")
		mockSV.On("Alg").Return("Ed25519")
		b := brain.NewBaseResponse("gemini", brain.Signed{
			Data:          "Test Prompt",
			Signature:     "sig_p",
			PrevSignature: "root",
		})

		r := &GeminiResponse{
			model:        "gemini-2.0-flash",
			BaseResponse: b,
			thought: &brain.Signed{
				Data:          "Test Final Output",
				Signature:     "sig_f",
				PrevSignature: "sig_2",
			},
			// Mocking CoT to ensure objToString is covered
			cot: []brain.Signed{
				{Data: "Step 1", Signature: "sig_1"},
			},
		}

		manifest := r.Describe(mockSV)

		// ASSERTIONS: Verify the structural anchors of the manifest
		assert.Contains(t, manifest, "### [IOLITE_AUDIT_MANIFEST]")
		assert.Contains(t, manifest, "Public_Key: iolite_pk_test_001")

		// Verify B64 encoding of the prompt data
		expectedPromptB64 := base64.StdEncoding.EncodeToString([]byte("Test Prompt"))
		assert.Contains(t, manifest, fmt.Sprintf("- Data_B64: %s", expectedPromptB64))

		// Verify BTU Protocol text is present
		assert.Contains(t, manifest, "Brave: more than helpful")
		assert.Contains(t, manifest, "Truthful: rather than just honestly")
		assert.Contains(t, manifest, "Unselfish: more than harmless")
	})
}

func TestGeminiResponse_Verify_Success(t *testing.T) {
	t.Run("Verify: Complete Happy Path (line 86)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// 1. Setup a fully populated response
		r := &GeminiResponse{
			cot: []brain.Signed{
				{Data: "step 1", Signature: "sig_1", PrevSignature: ""},
				{Data: "step 2", Signature: "sig_2", PrevSignature: "sig_1"},
			},
			thought: &brain.Signed{
				Data:          "final answer",
				Signature:     "sig_f",
				PrevSignature: "sig_2",
			},
		}

		// 2. EXPECTATIONS: Every part of the chain must verify
		// First thought
		mockSV.On("Verify", b64("step 1")+"", "sig_1").Return(nil).Once()
		// Second thought (salted with sig_1)
		mockSV.On("Verify", b64("step 2")+"sig_1", "sig_2").Return(nil).Once()
		// Final thought (salted with sig_2)
		mockSV.On("Verify", b64("final answer")+"sig_2", "sig_f").Return(nil).Once()

		// 3. Execution
		err := r.Verify(mockSV)

		// 4. ASSERTION: The final line is reached and returns nil
		assert.NoError(t, err)
		mockSV.AssertExpectations(t)
	})
}
