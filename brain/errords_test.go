package brain

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorDecision_Interface(t *testing.T) {
	initialErr := errors.New("original_sin")
	ed := &ErrorDecision{E: initialErr}

	t.Run("Error text is consistent", func(t *testing.T) {
		assert.Equal(t, "system error: "+initialErr.Error(), ed.Error())
	})
	t.Run("Map Return Safety", func(t *testing.T) {
		// Nil-path checks: Ensure Cots and Prompts don't crash and return nil
		assert.Nil(t, ed.Cots(), "Cots should be a terminal nil in ErrorDecision")
		assert.Nil(t, ed.Prompts(), "Prompts should be a terminal nil in ErrorDecision")
	})

	t.Run("Texts Generation Logic", func(t *testing.T) {
		texts := ed.Texts()

		require.NotNil(t, texts)
		require.Contains(t, texts, "system")

		systemBlocks := texts["system"]
		assert.Len(t, systemBlocks, 1)

		// Verify the string formatting is correct
		expectedData := fmt.Sprintf("CRITICAL_FAILURE: %v", initialErr)
		assert.Equal(t, expectedData, systemBlocks[0].Data)
		assert.Equal(t, TypeText, systemBlocks[0].Namespace)
	})

	t.Run("Add: Recursive Error Wrapping", func(t *testing.T) {
		ed := &ErrorDecision{E: errors.New("root_cause")}

		// Simulate adding a layer of failure context
		contextBlock := Signed{Data: "verification_failed"}

		// Note: ErrorDecision.Add ignores cot and sv, but we provide them for interface parity
		err := ed.Add("ignored_key", nil, contextBlock, nil)

		assert.NoError(t, err, "Add should return nil even if it wraps an internal error")

		// Verify the error wrapping logic: "text.Data: original_error"
		currentErrStr := ed.Unwrap().Error()
		assert.Contains(t, currentErrStr, "verification_failed")
		assert.Contains(t, currentErrStr, "root_cause")

		// Check unwrapping (standard Go error behavior)
		assert.True(t, errors.Is(ed.Unwrap(), ed.E))
	})

	t.Run("No-Op Safety", func(t *testing.T) {
		// Ensure these don't panic and return nil as expected
		assert.NoError(t, ed.Sign(nil))
		assert.NoError(t, ed.Verify(nil))
	})
}

func TestErrorResponse_Branches(t *testing.T) {
	// Setup
	baseErr := errors.New("sentinel_failure")
	errResp := &ErrorResponse{E: baseErr}
	mockVerifier := new(MockSignVerifier)

	// 1. Validate Core Data Methods
	t.Run("Data_Retrieval", func(t *testing.T) {
		assert.Nil(t, errResp.CoT(mockVerifier), "CoT should return nil for ErrorResponse")

		textResult := errResp.Text()
		assert.NotNil(t, textResult)
		assert.Equal(t, baseErr.Error(), textResult.Data)

		promptResult := errResp.Prompt()
		assert.Equal(t, baseErr.Error(), promptResult.Data)
	})

	// 2. Validate Interface Satisfactions (Sign/Verify/Describe)
	t.Run("Security_Interface", func(t *testing.T) {
		assert.Equal(t, baseErr.Error(), errResp.Describe(mockVerifier))
		assert.Equal(t, baseErr, errResp.Sign(mockVerifier), "Sign must return internal error")
		assert.Equal(t, baseErr, errResp.Verify(mockVerifier), "Verify must return internal error")
	})

	// 3. Validate Error Interface & Formatting
	t.Run("Error_Formatting", func(t *testing.T) {
		assert.Equal(t, baseErr.Error(), errResp.Source())

		expectedFormat := fmt.Sprintf("system error: %v", baseErr)
		assert.Equal(t, expectedFormat, errResp.Error())
	})

	// 4. Validate Unwrap and Refactored IsError Sentinel
	t.Run("Sentinel_Logic", func(t *testing.T) {
		// Verify Unwrap for errors.Is/As compatibility
		assert.Equal(t, baseErr, errors.Unwrap(errResp))

		// Verify the Iolite-specific IsError refactor
		assert.Equal(t, baseErr, errResp.IsError())
	})
}
