package brain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestClaudeResponse_Getters(t *testing.T) {
	t.Run("Prompt: Verify Pass-through", func(t *testing.T) {
		expected := Signed{Data: "System Directive", Namespace: TypePrompt}
		r := &BaseResponse{source: "test", prompt: expected}

		assert.Equal(t, expected, r.Prompt())
	})

	t.Run("Source: Identity Check", func(t *testing.T) {
		randstring := uuid.NewString()
		sv := new(MockSignVerifier)
		r, err := NewBaseResponse(sv, randstring, Signed{})
		assert.NoError(t, err)
		assert.Equal(t, randstring, r.Source())
	})
}
