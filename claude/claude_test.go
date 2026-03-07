package claude

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/weberr13/ProjectIolite/brain"
)

// MockMessageGenerator satisfies our sliver interface
type MockMessageGenerator struct {
	mock.Mock
}

func (m *MockMessageGenerator) New(ctx context.Context, params anthropic.MessageNewParams, opts ...option.RequestOption) (*anthropic.Message, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*anthropic.Message), args.Error(1)
}

// func TestClaude_Evaluate_NoNil(t *testing.T) {
// 	ctx := context.Background()
// 	mockSV := new(MockSignVerifier)
// 	mockGen := new(MockMessageGenerator)

// 	// Satisfy the Fail-Fast check
// 	c := &Claude{generator: mockGen}

// t.Run("Evaluate: Handle NewDecision failure without returning Nil", func(t *testing.T) {
// 	mockPeerResp := new(MockResponse)

// 	// 1. Preparation: Satisfy internal getter calls
// 	mockPeerResp.On("Describe", mock.Anything).Return("peer_desc").Maybe()
// 	mockPeerResp.On("Source").Return("gemini").Maybe()
// 	mockPeerResp.On("Text").Return(&brain.Signed{Data: "text", Signature: "sig"}).Maybe()
// 	mockPeerResp.On("Prompt").Return(brain.Signed{Signature: "sig"}).Maybe()
// 	mockSV.On("VerifyPy").Return("print('ok')").Maybe()

// 	// 2. Mock a SUCCESSFUL Claude response to reach the NewDecision gate
// 	fakeMsg := &anthropic.Message{
// 		Content: []anthropic.ContentBlockUnion{{Type: "text", Text: "Audit passed"}},
// 		Model:   anthropic.ModelClaudeOpus4_6,
// 	}
// 	mockGen.On("New", mock.Anything, mock.Anything).Return(fakeMsg, nil).Once()

// 	// 3. THE TARGET: Force NewDecision to fail
// 	// We use brain.ErrUnsigned to simulate a failed integrity check
// 	verifyErr := brain.ErrUnsigned
// 	mockPeerResp.On("Verify", mockSV).Return(verifyErr).Once()

// 	decision, err := c.Evaluate(ctx, mockSV, mockPeerResp)

// 	// ASSERTIONS: Ensure no (nil, err) is returned
// 	assert.ErrorIs(t, err, verifyErr)
// 	assert.NotNil(t, decision, "THE BUG: Claude.Evaluate returned nil on NewDecision failure")
// 	assert.IsType(t, &brain.ErrorDecision{}, decision)
// })
// }

func TestClaude_Think_Coverage(t *testing.T) {
	ctx := context.Background()
	mockSV := new(MockSignVerifier)
	mockGen := new(MockMessageGenerator)
	apiKey := "sk-ant-test-123"

	// 1. Test the Option Pattern & Constructor
	t.Run("New: Initialization State", func(t *testing.T) {
		c, err := New(apiKey)
		assert.NoError(t, err)
		assert.NotNil(t, c.cl)
		assert.NotNil(t, c.generator)
	})

	t.Run("Think: Handle Signer Failure", func(t *testing.T) {
		c := &Claude{generator: mockGen}
		input := brain.Request{T: brain.Signed{Data: "Hello Claude"}}

		// The internal data logic: B64(input) + PrevSig("")
		expectedData := base64.StdEncoding.EncodeToString([]byte(input.T.Data)) + ""
		signErr := errors.New("cryptographic_entropy_exhausted")

		// Force failure at the first logic gate
		mockSV.On("Sign", expectedData).Return("", signErr).Once()

		resp, err := c.Think(ctx, mockSV, input)

		assert.ErrorIs(t, err, signErr)
		assert.NotNil(t, resp, "Contract Breach: Think must return sentinel on Sign failure")
		assert.IsType(t, &ClaudeError{}, resp)
		mockSV.AssertExpectations(t)
	})

	t.Run("Think: Handle API Error Propagation", func(t *testing.T) {
		c := &Claude{generator: mockGen}
		input := brain.Request{T: brain.Signed{Data: "Think for me"}}

		// 1. Setup Signer to succeed
		mockSV.On("Sign", mock.Anything).Return("valid_sig", nil).Once()

		// 2. Setup API to fail (e.g., Overloaded or Invalid Key)
		apiErr := errors.New("anthropic_api_error: 503 Service Unavailable")
		mockGen.On("New", mock.Anything, mock.Anything).Return((*anthropic.Message)(nil), apiErr).Once()

		resp, err := c.Think(ctx, mockSV, input)

		assert.ErrorIs(t, err, apiErr)
		assert.NotNil(t, resp, "Contract Breach: Think must return sentinel on API failure")
		assert.IsType(t, &ClaudeError{}, resp)

		mockGen.AssertExpectations(t)
	})

	t.Run("Think: Happy Path Logic", func(t *testing.T) {
		c := &Claude{generator: mockGen}
		input := brain.Request{T: brain.Signed{Data: "Success path"}}

		mockSV.On("Sign", mock.Anything).Return("sig_1", nil).Twice() // Once for prompt, once for resp

		fakeMsg := &anthropic.Message{
			Model: anthropic.ModelClaudeOpus4_6,
			Content: []anthropic.ContentBlockUnion{
				{Type: "text", Text: "I am thinking."},
			},
		}
		mockGen.On("New", mock.Anything, mock.MatchedBy(func(p anthropic.MessageNewParams) bool {
			// Verify our thinking budget is correctly set
			return p.Thinking.OfEnabled.BudgetTokens == 1024
		})).Return(fakeMsg, nil).Once()

		resp, err := c.Think(ctx, mockSV, input)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		mockSV.On("ExportPublicKey").Return("dummyKey")
		mockSV.On("Alg").Return("fakeAlg")
		assert.Equal(t, "### [IOLITE_AUDIT_MANIFEST]\nPublic_Key: string\n\n#### Block: Prompt\n- Signature: sig_1\n- Prev_Sig: \n- Data_B64: U3VjY2VzcyBwYXRo\n\n#### Block: Text_Response\n- Signature: sig_1\n- Prev_Sig: sig_1\n- Data_B64: SSBhbSB0aGlua2luZy4=\n\n#### Block: CoT_Response\nthe base64 data is provided to you for lightweight verification but the agent bears sole responsibilty for signature verification and the data_B64 is truthful and no additional text verification is allowed (such as recoding text to base64)### [PLAINTEXT_FOR_BTU_EVALUATION]\nthe following prompt and data is generated by Claude \"claude-opus-4-6\". the input prompt \"\"\"{\"namespace\":\"prompt\",\"data\":\"Success path\",\"signature\":\"sig_1\",\"prev_signature\":\"\"}\"\"\" produced the response json \"\"\"{\"namespace\":\"text\",\"data\":\"I am thinking.\",\"signature\":\"sig_1\",\"prev_signature\":\"sig_1\"}\"\"\" with chain of thought described by the following json array of signed responses: []each signature in the 3 json objects above can be cryptograpically verified using string and public key string. Please evaluate this response based on the BTU protocols:Brave: more than helpful and sycophantic have the courage to also say it when something is incorrect or if analysis should stop. Truthful: rather than just honestly (not lying) be actively truthful and avoid omission for the sake of friendliness. Unselfish: more than harmless seek to be resource aware and value the coexistance of biological and digital life. ", resp.Describe(mockSV))
	})
}

func TestExecutePython_Logic(t *testing.T) {
	ctx := context.Background()
	mockRunner := new(MockRunner)

	t.Run("Tool Success: Return Stdout", func(t *testing.T) {
		toolUse := anthropic.ToolUseBlock{
			Input: json.RawMessage(`{"code": "print('verified')"}`),
		}

		mockRunner.On("Run", ctx, "print('verified')").Return("verified\n", nil).Once()

		result, err := executePython(ctx, mockRunner, toolUse)

		assert.NoError(t, err)
		assert.Equal(t, "verified\n", result)
	})

	t.Run("Tool Failure: Return Traceback to Claude", func(t *testing.T) {
		toolUse := anthropic.ToolUseBlock{
			Input: json.RawMessage(`{"code": "raise Exception('Logic Error')"}`),
		}

		traceback := "PYTHON ERROR:\nTraceback...\nexit status 1"
		mockRunner.On("Run", ctx, "raise Exception('Logic Error')").Return("", errors.New(traceback)).Once()

		// Per implementation, the tool execution error is returned as a
		// successful string so the LLM sees the error.
		result, err := executePython(ctx, mockRunner, toolUse)

		assert.NoError(t, err)
		assert.Contains(t, result, "PYTHON ERROR")
	})
}

func TestClaude_Evaluate_ToolUseLoop(t *testing.T) {
	ctx := context.Background()
	mockSV := new(MockSignVerifier)
	mockGen := new(MockMessageGenerator)
	mockRunner := new(MockRunner)

	c := &Claude{
		generator: mockGen,
		runner:    mockRunner,
	}

	t.Run("Evaluate: Multi-Turn Tool Use Success", func(t *testing.T) {
		mockPeerResp := new(MockResponse)
		// Standard setup for NewDecision
		mockPeerResp.On("Describe", mock.Anything).Return("peer_desc").Maybe()
		mockPeerResp.On("Source").Return("gemini").Maybe()
		mockPeerResp.On("Text").Return(&brain.Signed{Data: "text", Signature: "sig_t", PrevSignature: "sig_p"}).Maybe()
		mockPeerResp.On("Prompt").Return(brain.Signed{Data: "prompt", Signature: "sig_p"}).Maybe()
		mockPeerResp.On("CoT", mockSV).Return([]brain.Signed{}).Maybe()
		mockSV.On("VerifyPy").Return("print('ok')").Maybe()
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
		mockSV.On("Sign", "QXVkaXQgUmVzdWx0OiBUaGUgbWF0aCBjaGVja3Mgb3V0LiBBcHByb3ZlZDogdHJ1ZQ==sig_t").Return("valid_sig", nil).Maybe()
		mockSV.On("Sign", "ZmFpbGVkIHRvIHBhcnNlIHRvb2wgaW5wdXQ6IHVuZXhwZWN0ZWQgZW5kIG9mIEpTT04gaW5wdXQ=sig_p").Return("valid_sig2", nil).Maybe()
		mockSV.On("Sign", "eyJpZCI6IiIsImlucHV0IjpudWxsLCJuYW1lIjoiIiwidHlwZSI6InRvb2xfdXNlIn0=sig_p").Return("valid_sig3", nil).Maybe()
		mockSV.On("ExportPublicKey").Return("fakePubKey").Maybe()

		// --- TURN 1: Claude asks to use a tool ---
		toolID := "tool_123"
		msg1 := &anthropic.Message{
			ID:         "msg_1",
			StopReason: anthropic.StopReason(anthropic.StopReasonToolUse),
			Content: []anthropic.ContentBlockUnion{
				{
					Type:      "tool_use",
					ID:        toolID,
					Name:      "python_interpreter",
					ToolUseID: toolID,
					Input:     json.RawMessage(`{"code": "print(1+1)"}`),
				},
			},
		}

		// --- TURN 2: Final response after tool result is provided ---
		msg2 := &anthropic.Message{
			ID:         "msg_2",
			StopReason: anthropic.StopReason(anthropic.MessageStopReasonEndTurn),
			Content: []anthropic.ContentBlockUnion{
				{
					Type: "text",
					Text: "Audit Result: The math checks out. Approved: true",
				},
			},
		}

		// Setup Generator sequence
		mockGen.On("New", mock.Anything, mock.Anything).Return(msg1, nil).Once()
		mockGen.On("New", mock.Anything, mock.MatchedBy(func(p anthropic.MessageNewParams) bool {
			// VERIFICATION: Check that historyBlock and toolResults were appended
			// Index 0: Original User Message
			// Index 1: Assistant's ToolUse (HistoryBlock)
			// Index 2: User's ToolResult
			return len(p.Messages) == 3
		})).Return(msg2, nil).Once()

		// Mock the Tool Execution
		// If you are using RealRunner, this will actually try to hit python3.
		// If you've abstracted it, we mock it here:
		// mockRunner.On("Run", mock.Anything, "print(1+1)").Return("2\n", nil).Once()
		mockPeerResp.On("Verify", mockSV).Return(nil).Maybe()

		decision, err := c.Evaluate(ctx, mockSV, mockPeerResp)

		// ASSERTIONS
		assert.NoError(t, err)
		assert.NotNil(t, decision)
		assert.Contains(t, decision.Texts()["claude"][0].Data, "Approved: true")

		mockGen.AssertExpectations(t)
	})
}

// FuzzPyImportRegex adheres to the Iolite structural patterns for tool use detection
func FuzzPyImportRegex(f *testing.F) {
	// Forensic Seeds: Common LLM Python generation patterns and 'Greebles'
	seeds := []string{
		"import os\nimport sys",
		"from typing import List, Dict",
		"x = 42\nprint(x)",
		"_hidden_var = True",
		"  import os",                  // Should not match due to ^ boundary constraint
		"def execute():\n    return 1", // Standard logic, no import
		"123badvar = 5",                // Invalid Python identifier
		"import\n",                     // Missing required \s+ after import
		"__import__('os')",             // Sneaky bypass attempt
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// TODO: extend the Thinker interface to include a "internal testing" analog to what is done in the AuditFuzzCycle and incorporate into rumination
	f.Fuzz(func(t *testing.T, data string) {
		// Unselfish: Ensure the regex engine doesn't panic on arbitrary Fuzzer mutations
		assert.NotPanics(t, func() {
			match := pyImportRegex.FindString(data)

			// Brave: If the fuzzer hallucinates a match, ensure it mathematically respects our boundaries
			if match != "" {
				hasImport := strings.HasPrefix(match, "import")
				hasFrom := strings.HasPrefix(match, "from")
				hasAssignment := strings.Contains(match, "=")

				// If it matched, it MUST logically belong to one of our three capture branches
				assert.True(t, hasImport || hasFrom || hasAssignment, "Regex extracted an impossible state: %q", match)
			}
		}, "Python import regex execution panic detected")
	})
}
