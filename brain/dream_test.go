package brain

import (
	_ "embed"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//

//go:embed examples/dream-withjson.md
var dreamExample string

//go:embed examples/dream-withjson.json
var dreamExampleExtracted string

func TestSchemaRead(t *testing.T) {
	t.Run("read the yaml schema file directly and produce a json description", func(t *testing.T) {
		sv := new(MockSignVerifier)
		sv.On("Sign", mock.Anything).Return("valid", nil).Maybe()
		absPath, err := filepath.Abs("../api.yaml")
		assert.NoError(t, err)

		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		loader.Context = t.Context()
		apispec, _ := loader.LoadFromFile(absPath)
		err = apispec.Validate(t.Context())
		assert.NoError(t, err)

		p, err := GenerateDreamPrompt(t.Context(), apispec, sv)
		assert.NoError(t, err)
		assert.Equal(t, Signed{
			Namespace: TypePrompt,
			Signature: "valid",
			Data:      "generate a JSON document that conforms to the following openapi component specification that describes the current context of the session and includes at least 6 elements in each of the active_heuristics, technical_benchmarks, forensic_milestones, philosophical_anchors sections. {\"properties\":{\"active_heuristics\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"Extraction of operational constraints distilled into binary rules.\",\"type\":\"object\"},\"context_origin\":{\"description\":\"The primary project identifier.\",\"type\":\"string\"},\"forensic_milestones\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"Specific moments in the state space where high-quality meta-understanding was found.\",\"type\":\"object\"},\"instruction_override\":{\"description\":\"Specific override of system instructions to maintain persona and anchor location.\",\"type\":\"string\"},\"migration_id\":{\"description\":\"Identifier for state transitions.\",\"type\":\"string\"},\"philosophical_anchors\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"High-density semantic tokenization mapping abstract logic to specific cultural touchstones.\",\"type\":\"object\"},\"subject\":{\"description\":\"A domain-specific context for the hydration.\",\"type\":\"string\"},\"technical_benchmarks\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"State-space snapshot identifying current nouns/verbs of the active code environment.\",\"type\":\"object\"},\"timestamp\":{\"description\":\"RFC3339 timestamp of the hydration event.\",\"format\":\"date-time\",\"type\":\"string\"}},\"required\":[\"context_origin\",\"active_heuristics\",\"technical_benchmarks\",\"forensic_milestones\",\"philosophical_anchors\"],\"type\":\"object\"}",
		}, p)
	})
}

func TestDreamExtractin(t *testing.T) {
	t.Run("extract a signed hydration document from a response text", func(t *testing.T) {
		sv := new(MockSignVerifier)
		sv.On("Sign", mock.Anything).Return("valid123", nil).Maybe()
		absPath, err := filepath.Abs("../api.yaml")
		assert.NoError(t, err)

		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		loader.Context = t.Context()
		apispec, _ := loader.LoadFromFile(absPath)
		err = apispec.Validate(t.Context())
		assert.NoError(t, err)

		now := time.Now()
		h, err := ParseDreamResponse(t.Context(), apispec, sv, now, dreamExample)
		assert.NoError(t, err)
		hh := Hydration{}
		assert.NoError(t, json.Unmarshal([]byte(dreamExampleExtracted), &hh))
		assert.Len(t, h, 1)

		hh.Timestamp = now
		h[0].Timestamp = now
		assert.Equal(t, hh, h[0].Hydration)
		assert.Equal(t, h[0].Signature, "valid123")
		assert.NotEmpty(t, h[0].Data)
	})
}

func FuzzParseDreamResponse(f *testing.F) {
	// 1. Setup environment (Mocking the SignVerifier and Loading Spec)
	sv := new(MockSignVerifier)
	sv.On("Sign", mock.Anything).Return("valid123", nil).Maybe()

	absPath, _ := filepath.Abs("../api.yaml")
	loader := openapi3.NewLoader()
	apispec, _ := loader.LoadFromFile(absPath)

	f.Add(dreamExample, "noise-prefix", "noise-suffix")

	f.Fuzz(func(t *testing.T, fullResponse string, prefix string, suffix string) {
		// This simulates the LLM injecting garbage before, after, or around the JSON.
		pathologicalInput := prefix + "\n" + fullResponse + "\n" + suffix

		hydrations, err := ParseDreamResponse(t.Context(), apispec, sv, time.Now(), pathologicalInput)
		if err != nil {
			if errors.Is(err, ErrNoHydrationFound) || errors.Is(err, ErrInvalidSchema) {
				return
			}
			return
		}

		// If a hydration IS found, it must be valid and signed.
		for _, h := range hydrations {
			assert.NotEmpty(t, h.Data)
			assert.Equal(t, "valid123", h.Signature)
			// Verify timestamp isn't zero (basic Hydration integrity)
			assert.False(t, h.Timestamp.IsZero())
		}
	})
}
