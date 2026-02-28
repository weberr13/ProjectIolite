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
