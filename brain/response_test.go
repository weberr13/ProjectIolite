package brain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestClaudeResponse_Getters(t *testing.T) {
	t.Run("Prompt: Verify Pass-through", func(t *testing.T) {
		expected := Signed{Data: "System Directive", Namespace: "prompt"}
		r := &BaseResponse{source: "test", prompt: expected}

		assert.Equal(t, expected, r.Prompt())
	})

	t.Run("Source: Identity Check", func(t *testing.T) {
		randstring := uuid.NewString()
		r := NewBaseResponse(randstring, Signed{})
		assert.Equal(t, randstring, r.Source())
	})
}
