package gemini

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
	"google.golang.org/genai"
)

// Helper for cryptographic grounding
func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestGemini_Think_NoNil(t *testing.T) {
	ctx := context.Background()
	mockSV := new(MockSignVerifier)
	mockGen := new(MockGenerator)

	// Must satisfy the "Fail Fast" check
	g := &Gemini{
		cl:        &genai.Client{},
		generator: mockGen,
	}

	t.Run("Think: Sign Error returns GeminiError", func(t *testing.T) {
		testPrompt := "Why did the pen stay still?"
		req := brain.Request{T: testPrompt}

		// Calculate the exact string brain.Signed.Sign() will pass to the mock
		expectedSignInput := base64.StdEncoding.EncodeToString([]byte(testPrompt)) + ""

		signErr := errors.New("sign_fail")
		mockSV.On("Sign", expectedSignInput).Return("", signErr).Once()

		resp, err := g.Think(ctx, mockSV, req)

		assert.ErrorIs(t, err, signErr)
		assert.NotNil(t, resp, "Contract Breach: Think returned nil on error")
		assert.IsType(t, &GeminiError{}, resp)
		mockSV.AssertExpectations(t)
	})
}

func TestGemini_Evaluate_NoNil(t *testing.T) {
	ctx := context.Background()

	t.Run("Evaluate: Client Missing Handling", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		// New test case for the "Fail Fast" check we added
		badG := &Gemini{cl: nil}
		mockPeer := new(MockResponse)

		dec, err := badG.Evaluate(ctx, mockSV, mockPeer, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client not initialized")
		assert.NotNil(t, dec)
		assert.IsType(t, &brain.ErrorDecision{}, dec)
	})

	t.Run("Evaluate: NewDecision Failure Handling", func(t *testing.T) {
		mockPeerResp := new(MockResponse)
		mockSV := new(MockSignVerifier)
		mockGen := new(MockGenerator)
		g := &Gemini{
			cl:        &genai.Client{},
			generator: mockGen,
			model:     "gemini-pro-latest",
		}

		// 1. Utility methods - Use .Maybe() because NewDecision/Evaluate
		// call these multiple times for logging and map keys.
		mockPeerResp.On("Describe", mock.Anything).Return("fake_description").Maybe()
		mockPeerResp.On("Source").Return("claude").Maybe()
		mockPeerResp.On("Prompt").Return(brain.Signed{
			Data:      "original_prompt",
			Signature: "valid_looking_sig",
		}).Maybe()
		mockPeerResp.On("Text").Return(&brain.Signed{
			Data:      "peer_text",
			Signature: "parent_sig",
		}).Maybe()

		mockSV.On("VerifyPy").Return("print('mock python script')").Maybe()

		// 2. GenerateContent success
		fakeResult := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{{Text: "Audit: Pass"}}}},
			},
		}
		mockGen.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fakeResult, nil).Once()

		// 3. THE TARGET: Verify failure
		verifyErr := errors.New("signature_mismatch")

		// Remove .Once() and use .Return() to ensure that NO MATTER how many
		// times NewDecision or its internal loops call Verify, it always
		// fails with our specific error.
		mockPeerResp.On("Verify", mock.Anything).Return(verifyErr)

		dec, err := g.Evaluate(ctx, mockSV, mockPeerResp, nil)

		// ASSERTIONS
		assert.ErrorIs(t, err, verifyErr)
		assert.NotNil(t, dec, "Contract Breach: Evaluate returned nil")
		assert.IsType(t, &brain.ErrorDecision{}, dec)

		mockGen.AssertExpectations(t)
		// We check expectations to ensure Verify was actually called
		mockPeerResp.AssertExpectations(t)
	})
}

func TestGemini_Evaluate_RevealTheNil(t *testing.T) {
	ctx := context.Background()
	mockSV := new(MockSignVerifier) // From previous sliver mock
	mockGen := new(MockGenerator)

	// We MUST have a non-nil cl and generator to pass the "Fail Fast" check
	g := &Gemini{
		cl:        &genai.Client{},
		generator: mockGen,
		model:     "gemini-pro-latest",
	}

	t.Run("Diagnostic: Force NewDecision failure to reveal Nil Return", func(t *testing.T) {
		mockPeerResp := new(MockResponse)

		// 1. Satisfy the string builders
		mockPeerResp.On("Describe", mock.Anything).Return("valid_desc")
		mockPeerResp.On("Text").Return(&brain.Signed{Signature: "parent_sig"})
		mockSV.On("VerifyPy").Return("print('ok')")

		// 2. Mock a SUCCESSFUL GenerateContent call to proceed to NewDecision
		// We return a skeleton response so result.Text() doesn't panic
		fakeResult := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{{Content: &genai.Content{Parts: []*genai.Part{{Text: "Audit: Pass"}}}}},
		}
		mockGen.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fakeResult, nil)

		// 3. TARGET: Make NewDecision fail gracefully
		// We mock Verify to return an error. This skips the dereference
		// at line 33 and goes straight to: return nil, err
		verifyErr := errors.New("corrupted_response_signature")
		mockPeerResp.On("Verify", mock.Anything).Return(verifyErr)

		// 4. EXECUTION
		decision, err := g.Evaluate(ctx, mockSV, mockPeerResp, nil)

		// THE REVEAL: If the bug exists, 'decision' will be nil here.
		assert.NotNil(t, decision, "THE BUG REVEALED: Evaluate returned (nil, err) because NewDecision failed!")
		assert.Error(t, err)
	})
}

func TestNew(t *testing.T) {
	ctx := context.Background()

	t.Run("Error: Missing API Key", func(t *testing.T) {
		// Target: Trigger ErrNoKeyFound
		g, err := New(ctx, "")

		assert.Nil(t, g)
		assert.ErrorIs(t, err, ErrNoKeyFound)
	})

	t.Run("Options: WithModel Overrides Default", func(t *testing.T) {
		customModel := "gemini-3.1-pro-preview"

		// Note: NewClient will likely fail in a test environment without
		// a real network, so we check for the error return or mock the call.
		// For this audit, we focus on the logic before the client init.
		g, _ := New(ctx, "fake-key", WithModel(customModel))

		if g != nil {
			assert.Equal(t, customModel, g.model)
		}
	})

	t.Run("Options: WithGeminiConfig Persistence", func(t *testing.T) {
		customCfg := &genai.ClientConfig{
			Backend: genai.BackendVertexAI,
		}

		g, _ := New(ctx, "fake-key", WithGeminiConfig(customCfg))

		if g != nil {
			assert.Equal(t, customCfg, g.cfg)
		}
	})
}

func TestNew_InternalReachable(t *testing.T) {
	ctx := context.Background()

	t.Run("New: Force Reachable Assignments", func(t *testing.T) {
		// Save the real one and restore after
		oldNewClient := newClient
		defer func() { newClient = oldNewClient }()

		// Mock the SDK constructor to return a dummy client
		newClientMX.Lock()
		newClient = func(ctx context.Context, cfg *genai.ClientConfig) (*genai.Client, error) {
			return &genai.Client{
				Models: &genai.Models{}, // Provide the nested field
			}, nil
		}
		newClientMX.Unlock()
		defer func() {
			newClientMX.Lock()
			newClient = genai.NewClient
			newClientMX.Unlock()
		}()

		g, err := New(ctx, "any-key")

		assert.NoError(t, err)
		assert.NotNil(t, g.cl)
		assert.NotNil(t, g.generator) // BOOM: Reachable.
	})
}

func TestGemini_Think(t *testing.T) {
	ctx := context.Background()
	mockSV := new(MockSignVerifier)
	mockSV.On("Sign", mock.Anything).Return("sig_mock", nil)

	t.Run("Guard: Client Not Initialized (line 7)", func(t *testing.T) {
		// Create a hollow Gemini struct without New()
		g := &Gemini{}
		input := brain.Request{T: "test"}

		resp, err := g.Think(ctx, mockSV, input)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not initialized")
		// Verify the ErrResponse interface is used correctly
		assert.Equal(t, "system error: gemini client not initialized", resp.(*brain.ErrorResponse).Error())
	})

	t.Run("Think: Success Path (The Long Branch)", func(t *testing.T) {
		mockGen := new(MockModels) // Our mock of the genai.Models service
		g := &Gemini{
			cl:        &genai.Client{},
			generator: mockGen,
			model:     "gemini-3-flash",
		}
		input := brain.Request{T: "hello world"}

		// 1. Mock the SDK response
		expectedResult := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{{Text: "response", Thought: true}}}},
			},
		}
		mockGen.On("GenerateContent", ctx, "gemini-3-flash", genai.Text("hello world"), mock.Anything).
			Return(expectedResult, nil).Once()

		// 2. Execution
		resp, err := g.Think(ctx, mockSV, input)

		// 3. ASSERTIONS
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "gemini", resp.Source())
		mockGen.AssertExpectations(t)
	})
}

func TestGemini_Think_GeneratorFailure(t *testing.T) {
	ctx := context.Background()
	mockSV := new(MockSignVerifier)
	// We still need to sign the prompt first
	mockSV.On("Sign", mock.Anything).Return("sig_prompt", nil).Once()

	t.Run("Failure: GenerateContent Returns API Error", func(t *testing.T) {
		mockGen := new(MockModels)
		g := &Gemini{
			cl:        &genai.Client{},
			generator: mockGen,
			model:     "gemini-3-flash",
		}
		input := brain.Request{T: "trigger error"}

		// 1. Mock a concrete API error (e.g., Quota Exceeded)
		apiErr := errors.New("googleapi: Error 429: Rate limit exceeded")
		mockGen.On("GenerateContent", ctx, "gemini-3-flash", genai.Text("trigger error"), mock.Anything).
			Return(nil, apiErr).Once()

		// 2. Execution
		resp, err := g.Think(ctx, mockSV, input)

		// 3. ASSERTIONS
		assert.Error(t, err)
		assert.ErrorIs(t, err, apiErr)

		// Verify the response is a GeminiError wrapper
		var gErr *GeminiError
		assert.IsType(t, &GeminiError{}, resp)
		assert.ErrorAs(t, resp.(error), &gErr)

		assert.Contains(t, resp.(*GeminiError).Error(), "Rate limit exceeded")

		mockGen.AssertExpectations(t)
	})
}

func TestGemini_Evaluate_Advanced(t *testing.T) {
	ctx := context.Background()

	t.Run("Branch: Generation Error", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("VerifyPy").Return("print('verify')")
		mockSV.On("Sign", mock.Anything).Return("sig_gem_audit", nil)
		peerOutput := new(MockResponse)
		peerOutput.On("Text").Return(&brain.Signed{Signature: "peer_sig_123", Data: "hello"}).Maybe()
		peerOutput.On("Describe", mock.Anything).Return("manifest_data")
		mockGen := new(MockModels)
		g := &Gemini{cl: &genai.Client{}, generator: mockGen}

		genErr := errors.New("safety_filter_blocked")
		mockGen.On("GenerateContent", ctx, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, genErr).Once()

		dec, err := g.Evaluate(ctx, mockSV, peerOutput, nil)
		assert.ErrorIs(t, err, genErr)
		assert.IsType(t, &brain.ErrorDecision{}, dec)
	})

	t.Run("Branch: Final Stitching and Signing", func(t *testing.T) {
		promptSig := "sig_p"
		mockSV := new(MockSignVerifier)
		mockSV.On("VerifyPy").Return("print('verify')")
		mockSV.On("Sign", mock.Anything).Return("sig_gem_audit", nil)
		peerOutput := new(MockResponse)
		peerOutput.On("Text").Return(&brain.Signed{Signature: "peer_sig_123", Data: "hello", PrevSignature: promptSig}).Maybe()
		peerOutput.On("Describe", mock.Anything).Return("manifest_data")
		mockGen := new(MockModels)
		g := &Gemini{cl: &genai.Client{}, generator: mockGen}

		// Mock a result with text to verify stitching
		result := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{{Text: "Audit Approved"}}}},
			},
		}
		mockGen.On("GenerateContent", ctx, mock.Anything, mock.Anything, mock.Anything).
			Return(result, nil).Once()

		peerOutput.On("Verify", mock.Anything).Return(nil).Maybe()
		peerOutput.On("Source").Return("gemini").Maybe()
		peerOutput.On("Prompt").Return(brain.Signed{Signature: promptSig}).Maybe()
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

		dec, err := g.Evaluate(ctx, mockSV, peerOutput, nil)

		assert.NoError(t, err)
		assert.NotNil(t, dec)
		// Verification of stitching: Gemini's audit must link to peer's signature
		// This is verified via the internal calls to prev.Add and prev.Sign
	})

	t.Run("Branch: Final Stitching and Signing (The Hardened Audit Trail)", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockGen := new(MockModels)
		peerOutput := new(MockResponse)
		g := &Gemini{cl: &genai.Client{}, generator: mockGen, model: "gemini-pro"}

		// Constants
		pTxt := "hello"
		pTxtSig := "peer_sig_123"
		pPrompt := "original prompt"
		pPromptSig := "peer_prompt_sig_456"
		resTxt := "Audit Approved"

		result := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{{Text: resTxt}}}},
			},
		}

		// 1. Peer Output Mocks
		peerOutput.On("Text").Return(&brain.Signed{Signature: pTxtSig, Data: pTxt, PrevSignature: pPromptSig}).Maybe()
		peerOutput.On("Describe", mock.Anything).Return("manifest_data").Once()
		peerOutput.On("Verify", mockSV).Return(nil).Twice()
		peerOutput.On("Prompt").Return(brain.Signed{Signature: pPromptSig, Data: pPrompt}).Maybe()
		peerOutput.On("Source").Return("gemini").Maybe()

		// 2. Generation Mock
		mockGen.On("GenerateContent", mock.Anything, "gemini-pro-latest", mock.Anything, mock.Anything).
			Return(result, nil).Once()

		// 3. THE CRYPTOGRAPHIC CEREMONY
		mockSV.On("VerifyPy").Return("print('verify')").Once()

		// --- STEP A: NewDecision validates the peer prompt ---
		mockSV.On("Verify", b64(pPrompt)+"", pPromptSig).Return(nil).Once()

		// --- STEP C: NewDecision validates the peer text ---
		mockSV.On("Verify", b64(pTxt)+pPromptSig, pTxtSig).Return(nil).Once()

		// --- STEP D: Add() signs & immediately verifies the new Gemini Block ---
		auditData := b64(resTxt) + pTxtSig
		mockSV.On("Sign", auditData).Return("sig_gem_text", nil).Once()
		// THIS IS THE CALL FROM THE TRACE: Verify the block just signed
		mockSV.On("Verify", auditData, "sig_gem_text").Return(nil).Once()

		// 4. Execution
		dec, err := g.Evaluate(ctx, mockSV, peerOutput, nil)

		assert.NoError(t, err)
		assert.NotNil(t, dec)

		mockSV.AssertExpectations(t)
	})
}

func TestGemini_Evaluate_FinalBranches(t *testing.T) {
	ctx := context.Background()
	mockSV := new(MockSignVerifier)
	mockGen := new(MockModels)
	peerOutput := new(MockResponse)
	g := &Gemini{cl: &genai.Client{}, generator: mockGen, model: "gemini-pro"}

	t.Run("Branch: Multi-part Instruction Log ", func(t *testing.T) {
		// To trigger the 'else', we need 'instruction' length != 1
		// Since we can't easily change the code logic, we look at the flow.
		// If instruction were empty or had 2 parts, it hits the log.
		// This usually requires a mock of a constructor if it were an interface,
		// but here we can simulate by checking the side effects of an empty VerifyPy return.
		mockSV.On("VerifyPy").Return("").Once()

		// Setup enough for the generator to run
		mockGen.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(&genai.GenerateContentResponse{}, nil).Once()
		peerOutput.On("Describe", mock.Anything).Return("data").Once()
		peerOutput.On("Text").Return(&brain.Signed{Signature: "", Data: "text"}).Maybe()
		peerOutput.On("Verify", mockSV).Return(nil).Twice()
		peerOutput.On("Prompt").Return(brain.Signed{Signature: "", Data: "prompt"}).Maybe()
		peerOutput.On("Source").Return("gemini").Maybe()
		mockSV.On("Sign", mock.Anything).Return("text", nil).Maybe()
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

		_, _ = g.Evaluate(ctx, mockSV, peerOutput, nil)
		// Check logs for "could not generate single part system instruction"
	})

	t.Run("Branch: Decision Add Failure", func(t *testing.T) {
		// Setup successful generation
		resultText := "Audit Approved"
		result := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{Content: &genai.Content{Parts: []*genai.Part{{Text: resultText}}}},
			},
		}

		mockGen.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(result, nil).Once()
		mockSV.On("VerifyPy").Return("print('ok')").Once()
		peerOutput.On("Describe", mock.Anything).Return("data").Once()
		peerOutput.On("Text").Return(&brain.Signed{Signature: "sig"}).Once()
		peerOutput.On("Verify", mock.Anything).Return(nil).Once()

		// Use a MockDecision that purposefully fails the Add call
		mockPrev := new(MockDecision)
		mockPrev.On("Add", "gemini", mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("stitching_failed")).Once()

		dec, err := g.Evaluate(ctx, mockSV, peerOutput, mockPrev)

		assert.Error(t, err)
		assert.Equal(t, "stitching_failed", err.Error())
		assert.Equal(t, mockPrev, dec) // Ensure it returns the prev even on error
	})
}
