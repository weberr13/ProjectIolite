package brain

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//go:embed exampleDecision4.json
var decision4 string

type FakeDecision struct {
	ChainOfThoughts map[string][][]Signed
	AllPrompts      map[string][]Signed
	AllTexts        map[string][]Signed
}

func (f *FakeDecision) Texts() map[string][]Signed {
	return f.AllTexts
}

func TestDecisionParser_IsApproved_Example4(t *testing.T) {
	// 1. Setup Parser and Mock Verifier
	parser := &DecisionParser{}
	mockSV := new(MockSignVerifier)

	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
	// 2. Unmarshal example data into a concrete Decision implementation
	// Note: You may need to adapt this to your specific 'WholeDecision' struct
	var dec FakeDecision
	err := json.Unmarshal([]byte(decision4), &dec)
	require.NoError(t, err, "Failed to unmarshal exampleDecision4.json")

	// 3. Execute the public method
	// This will internally call parseForJsonBlocks on the 'claude' source (since it has sourceSig == "")
	approved, err := parser.IsApproved(mockSV, &dec)

	// 4. Assertions
	t.Run("BTU Consensus Result", func(t *testing.T) {
		assert.NoError(t, err)
		// Based on exampleDecision4.json, Claude outputs {"approved": true} at the end.
		// Note: The example uses "approved", while the parser looks for "accepted".
		// I will provide a fix for this discrepancy in the logic check below.
		assert.True(t, approved, "The parser should find the approved/accepted terminal state")
	})
}

func TestDecisionParser_CoverageBranches(t *testing.T) {
	parser := &DecisionParser{}

	t.Run("Verify_Failure_Branch", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		// Simulate a cryptographic break in the braid
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(errors.New("invalid signature"))

		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"claude": {{Data: "test", Signature: "sig1", PrevSignature: ""}},
			},
		}

		approved, err := parser.IsApproved(mockSV, dec)
		assert.Error(t, err)
		assert.False(t, approved)
	})

	t.Run("Malformed_JSON_Logging_Branch", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

		// One good block, one broken block to hit the log.Printf branch
		mixedData := "```json\n{\"accepted\": true}\n```\n```json\n{\"accepted\": \"not-a-bool\"}\n```"
		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "source", Signature: "sig1", PrevSignature: ""}},
				"claude": {{Data: mixedData, Signature: "sig2", PrevSignature: "sig1"}},
			},
		}

		approved, err := parser.IsApproved(mockSV, dec)
		assert.NoError(t, err)
		assert.True(t, approved, "Should skip bad JSON and find the valid 'true'")
	})

	t.Run("Consensus_Veto_Branch", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

		// Construct a conflict: One auditor accepts, another rejects.
		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini":  {{Data: "source", Signature: "sig", PrevSignature: ""}},
				"claude":  {{Data: "```json\n{\"accepted\": true}\n```", Signature: "sig2", PrevSignature: "sig"}},
				"arbiter": {{Data: "```json\n{\"accepted\": false}\n```", Signature: "sig3", PrevSignature: "sig2"}},
			},
		}

		approved, err := parser.IsApproved(mockSV, dec)
		assert.NoError(t, err)
		// This validates your `allAccepted = allAccepted && val` logic
		assert.False(t, approved, "Strict veto: a single 'false' must tank the approval")
	})
}

func TestDecisionParser_BraidChain(t *testing.T) {
	parser := &DecisionParser{}
	mockSV := new(MockSignVerifier)
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

	t.Run("Debate_Resolution_Last_Authoritative", func(t *testing.T) {
		// Block 1: Initial rejection by Team Red (Valor)
		b1 := Signed{
			Data:          "```json\n{\"approved\": false}\n```",
			Signature:     "sig_alpha",
			PrevSignature: "", // Genesis
		}
		// Block 2: Rebuttal/Refinement by Team Blue (Mystic)
		b2 := Signed{
			Data:          "We fixed the Snark integration.",
			Signature:     "sig_beta",
			PrevSignature: "sig_alpha",
		}
		// Block 3: Final Approval by Team Red
		b3 := Signed{
			Data:          "```json\n{\"accepted\": true}\n```",
			Signature:     "sig_gamma",
			PrevSignature: "sig_beta",
		}

		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini": {b1, b3},
				"claude": {b2},
			},
		}

		approved, err := parser.IsApproved(mockSV, dec)
		assert.NoError(t, err)
		// Even though Block 1 said false, Block 3 (the last in the chain) said true.
		assert.True(t, approved, "The braid should resolve to the last authoritative answer")
	})

	t.Run("No_Decision_Found_Branch", func(t *testing.T) {
		// A valid braid with no JSON blocks at all
		b1 := Signed{
			Data:          "Just some thoughts about the weather in Colorado.",
			Signature:     "sig_weather",
			PrevSignature: "",
		}

		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini": {b1},
			},
		}

		approved, err := parser.IsApproved(mockSV, dec)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no terminal decision found")
		assert.False(t, approved)
	})
}

func TestDecisionParser_GlobalBase64(t *testing.T) {
	parser := &DecisionParser{}
	mockSV := new(MockSignVerifier)
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

	t.Run("Full_Envelope_Base64_Branch", func(t *testing.T) {
		// The raw 'human' message
		rawMessage := "BTU Audit complete.\n```json\n{\"approved\": true}\n```"

		// The "All-or-Nothing" transport encoding
		encodedMessage := base64.StdEncoding.EncodeToString([]byte(rawMessage))

		b1 := Signed{
			Data:          encodedMessage, // The ENTIRE field is B64
			Signature:     "sig_envelope",
			PrevSignature: "",
		}

		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"claude": {b1},
			},
		}

		approved, err := parser.IsApproved(mockSV, dec)

		assert.NoError(t, err)
		assert.True(t, approved, "Should decode the entire envelope and find the JSON within")
	})
}
