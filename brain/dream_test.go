package brain

import (
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
			Data:      "generate a JSON document that conforms to the following openapi component specification that describes the current context of the session. {\"properties\":{\"active_heuristics\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"Extraction of operational constraints distilled into binary rules.\",\"type\":\"object\"},\"context_origin\":{\"description\":\"The primary project identifier.\",\"type\":\"string\"},\"forensic_milestones\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"Specific moments in the state space where high-quality meta-understanding was found.\",\"type\":\"object\"},\"instruction_override\":{\"description\":\"Specific override of system instructions to maintain persona and anchor location.\",\"type\":\"string\"},\"migration_id\":{\"description\":\"Identifier for state transitions.\",\"type\":\"string\"},\"philosophical_anchors\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"High-density semantic tokenization mapping abstract logic to specific cultural touchstones.\",\"type\":\"object\"},\"subject\":{\"description\":\"A domain-specific context for the hydration.\",\"type\":\"string\"},\"technical_benchmarks\":{\"additionalProperties\":{\"type\":\"string\"},\"description\":\"State-space snapshot identifying current nouns/verbs of the active code environment.\",\"type\":\"object\"},\"timestamp\":{\"description\":\"RFC3339 timestamp of the hydration event.\",\"format\":\"date-time\",\"type\":\"string\"}},\"required\":[\"context_origin\",\"active_heuristics\",\"technical_benchmarks\",\"forensic_milestones\",\"philosophical_anchors\"],\"type\":\"object\"}",
		}, p)
	})
}
