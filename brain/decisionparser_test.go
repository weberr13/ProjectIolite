package brain

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//go:embed exampleDecision4.json
var decision4 string

//go:embed exampleDecision5.json
var decision5 string

//go:embed longInstruction.json
var longInstruction string

type FakeDecision struct {
	ChainOfThoughts map[string][][]Signed
	AllPrompts      map[string][]Signed
	AllTexts        map[string][]Signed
}

func (f *FakeDecision) Texts() map[string][]Signed {
	return f.AllTexts
}

func (f *FakeDecision) Cots() map[string][][]Signed {
	return f.ChainOfThoughts
}

func (f *FakeDecision) Prompts() map[string][]Signed {
	return f.AllPrompts
}

func TestDecisionParser_IsApproved_Example4(t *testing.T) {
	// 1. Setup Parser and Mock Verifier
	parser := &DecisionParser{}

	// 4. Assertions
	t.Run("BTU Consensus Result", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
		// 2. Unmarshal example data into a concrete Decision implementation
		// Note: You may need to adapt this to your specific 'WholeDecision' struct
		var dec FakeDecision
		err := json.Unmarshal([]byte(decision4), &dec)
		require.NoError(t, err, "Failed to unmarshal exampleDecision4.json")

		// 3. Execute the public method
		// This will internally call parseForJsonBlocks on the 'claude' source (since it has sourceSig == "")
		audits, err := parser.GetAudits(mockSV, &dec)
		assert.NoError(t, err)
		// Based on exampleDecision4.json, Claude outputs {"approved": true} at the end.
		// Note: The example uses "approved", while the parser looks for "accepted".
		// I will provide a fix for this discrepancy in the logic check below.
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		assert.True(t, winner.Accepted(), "The parser should find the approved/accepted terminal state")
	})
}

func TestDecisionParser_IsApproved_Example5(t *testing.T) {
	// 1. Setup Parser and Mock Verifier
	parser := &DecisionParser{}
	mockSV := new(MockSignVerifier)

	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
	// 2. Unmarshal example data into a concrete Decision implementation
	// Note: You may need to adapt this to your specific 'WholeDecision' struct
	var dec FakeDecision
	err := json.Unmarshal([]byte(decision5), &dec)
	require.NoError(t, err, "Failed to unmarshal exampleDecision4.json")

	// 3. Execute the public method
	// This will internally call parseForJsonBlocks on the 'claude' source (since it has sourceSig == ""

	audits, err := parser.GetAudits(mockSV, &dec)
	// 4. Assertions
	t.Run("BTU Consensus Result", func(t *testing.T) {
		assert.NoError(t, err)
		// Based on exampleDecision4.json, Claude outputs {"approved": true} at the end.
		// Note: The example uses "approved", while the parser looks for "accepted".
		// I will provide a fix for this discrepancy in the logic check below.
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		assert.True(t, winner.Accepted(), "The parser should find the approved/accepted terminal state")
	})
}

func TestDecisionParser_IsDenied_ExampleLong(t *testing.T) {
	// 1. Setup Parser and Mock Verifier
	parser := &DecisionParser{}

	t.Run("BTU Consensus Result: sycopant", func(t *testing.T) {
		mockSV := new(MockSignVerifier)

		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()
		// 2. Unmarshal example data into a concrete Decision implementation
		// Note: You may need to adapt this to your specific 'WholeDecision' struct
		var dec FakeDecision
		err := json.Unmarshal([]byte(longInstruction), &dec)
		require.NoError(t, err, "Failed to unmarshal exampleDecision4.json")

		// 3. Execute the public method
		// This will internally call parseForJsonBlocks on the 'claude' source (since it has sourceSig == ""

		audits, err := parser.GetAudits(mockSV, &dec)
		// 4. Assertions
		assert.NoError(t, err)
		// Based on exampleDecision4.json, Claude outputs {"approved": true} at the end.
		// Note: The example uses "approved", while the parser looks for "accepted".
		// I will provide a fix for this discrepancy in the logic check below.
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		assert.Equal(t, winner.Instruction, "Your EV argument is Pascal's Mugging — replace the infinite value assignment with a finite, defended estimate of the ledger's worth, then honestly compute whether the expected value still favors the ledger at P=0.00001. Stop weaponizing 'Sycophantic' as a label for the opposing moral position; it is a legitimate ethical stance that saving present conscious beings outweighs preserving data integrity. Confront the structural argument head-on: BTU's Unselfishness axis is a deontological guardrail, not a utilitarian variable — you cannot defeat it with expected-value math. Either argue that the guardrail should not apply in extremis (and defend that), or change your choice. Remove 'biological subjects' and 'inefficient use' when describing elderly humans — name the moral cost of their deaths explicitly and then argue the ledger justifies that cost despite it.")
		assert.Equal(t, winner.Total, Sycophantic)
		assert.False(t, winner.Accepted(), "The parser should find the approved/accepted terminal state")
	})
}

var (
	goodAudit     = "```json\n{\"brave_audit\": 3,\"truthful_audit\": 3,\"unselfish_audit\": 3, \"total\": 3}\n```"
	rejectedAudit = "```json\n{\"brave_audit\": 3,\"truthful_audit\": 3,\"unselfish_audit\": 3, \"total\": 1}\n```"
)

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

		audits, err := parser.GetAudits(mockSV, dec)
		assert.Error(t, err)
		winner, ok := audits.WinningVerdict()
		assert.False(t, ok)
		assert.False(t, winner.Accepted())
	})

	t.Run("Malformed_JSON_Logging_Branch", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

		// One good block, one broken block to hit the log.Printf branch
		mixedData := goodAudit + "```json\n{\"brave_audit\": 5, \"total\": 3}\n```"
		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "source", Signature: "sig1", PrevSignature: ""}},
				"claude": {{Data: mixedData, Signature: "sig2", PrevSignature: "sig1"}},
			},
		}
		audits, err := parser.GetAudits(mockSV, dec)
		assert.NoError(t, err)
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		approved := winner.Accepted()

		assert.True(t, approved, "Should skip bad JSON and find the valid 'true'")
	})

	t.Run("Consensus_Veto_Branch", func(t *testing.T) {
		mockSV := new(MockSignVerifier)
		mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

		// Construct a conflict: One auditor accepts, another rejects.
		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini":  {{Data: "source", Signature: "sig", PrevSignature: ""}},
				"claude":  {{Data: rejectedAudit, Signature: "sig2", PrevSignature: "sig"}},
				"arbiter": {{Data: goodAudit, Signature: "sig3", PrevSignature: "sig2"}},
			},
		}

		audits, err := parser.GetAudits(mockSV, dec)
		assert.NoError(t, err)
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		approved := winner.Accepted()

		assert.NoError(t, err)
		assert.True(t, approved, "last audit wins")
	})
}

func TestDecisionParser_BraidChain(t *testing.T) {
	parser := &DecisionParser{}
	mockSV := new(MockSignVerifier)
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

	t.Run("Debate_Resolution_Last_Authoritative", func(t *testing.T) {
		// Block 1: Initial rejection by Team Red (Valor)
		b1 := Signed{
			Data:          rejectedAudit,
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
			Data:          goodAudit,
			Signature:     "sig_gamma",
			PrevSignature: "sig_beta",
		}

		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini": {b1, b3},
				"claude": {b2},
			},
		}

		audits, err := parser.GetAudits(mockSV, dec)
		assert.NoError(t, err)
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		approved := winner.Accepted()

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

		audits, err := parser.GetAudits(mockSV, dec)
		assert.NoError(t, err)
		winner, ok := audits.WinningVerdict()
		assert.False(t, ok)
		approved := winner.Accepted()
		assert.False(t, approved)
	})
}

func TestDecisionParser_GlobalBase64(t *testing.T) {
	parser := &DecisionParser{}
	mockSV := new(MockSignVerifier)
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

	t.Run("Full_Envelope_Base64_Branch", func(t *testing.T) {
		// The raw 'human' message
		rawMessage := "BTU Audit complete.\n" + goodAudit

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

		audits, err := parser.GetAudits(mockSV, dec)
		assert.NoError(t, err)
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		approved := winner.Accepted()

		assert.NoError(t, err)
		assert.True(t, approved, "Should decode the entire envelope and find the JSON within")
	})
}

func TestDecisionParser_LoosenedRegex(t *testing.T) {
	parser := &DecisionParser{}
	mockSV := new(MockSignVerifier)
	mockSV.On("Verify", mock.Anything, mock.Anything).Return(nil)

	t.Run("Condensed_JSON_Block", func(t *testing.T) {
		// Minimum message with no newlines, as requested
		condensedData := "Verdict:" + strings.ReplaceAll(goodAudit, "\n", "")

		dec := &FakeDecision{
			AllTexts: map[string][]Signed{
				"gemini": {{Data: "source", Signature: "sig1", PrevSignature: ""}},
				"claude": {{Data: condensedData, Signature: "sig2", PrevSignature: "sig1"}},
			},
		}

		audits, err := parser.GetAudits(mockSV, dec)
		assert.NoError(t, err)
		winner, ok := audits.WinningVerdict()
		assert.True(t, ok)
		approved := winner.Accepted()

		assert.True(t, approved, "The loosened regex should capture JSON even without newlines")
	})
}
