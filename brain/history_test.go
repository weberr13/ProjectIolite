package brain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestChatHistory_AppendInPlace(t *testing.T) {
	// GIVEN a history slice
	var history ChatHistory
	user := Signed{Data: "user prompt"}
	model := Signed{Data: "model response"}

	// WHEN we append in place
	history.AppendInPlace(user, model)

	// THEN the slice should persist the change (verifying the pointer receiver fix)
	assert.Equal(t, 1, len(history), "History should have exactly one node")
	assert.Equal(t, "user prompt", history[0].User.Data)
	assert.Equal(t, "model response", history[0].Model.Data)
}

func TestChatHistory_VerifyChain(t *testing.T) {
	sv := new(MockSignVerifier)

	// GIVEN a history with two turns
	history := ChatHistory{
		{User: Signed{Data: "u1", Signature: "s1"}, Model: Signed{Data: "m1", Signature: "s3"}},
		{User: Signed{Data: "u2", Signature: "s2"}, Model: Signed{Data: "m2", Signature: "s4"}},
	}

	// EXPECTATION: Verify called 4 times (2 nodes * 2 Signed blocks)
	sv.On("Verify", mock.Anything, mock.Anything).Return(nil).Times(4)

	// WHEN we verify the whole history
	err := history.Verify(sv)

	// THEN
	assert.NoError(t, err)
	sv.AssertExpectations(t)
}

func TestChatHistory_VerifyFailure_Prompt(t *testing.T) {
	sv := new(MockSignVerifier)

	// GIVEN a history where the second turn's model response is corrupted
	history := ChatHistory{
		{User: Signed{Data: "valid", Signature: "s1"}, Model: Signed{Data: "valid", Signature: "s1"}},
		{User: Signed{Data: "invalid", Signature: "BLAH!"}, Model: Signed{Data: "valid", Signature: "s1"}},
	}

	// EXPECTATION: Return error on the specific corrupted block
	sv.On("Verify", mock.Anything, "BLAH!").Return(errors.New("invalid signature"))
	sv.On("Verify", mock.Anything, mock.Anything).Return(nil) // Allow others to pass until failure

	// WHEN
	err := history.Verify(sv)

	// THEN
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestChatHistory_VerifyFailure_Response(t *testing.T) {
	sv := new(MockSignVerifier)

	// GIVEN a history where the second turn's model response is corrupted
	history := ChatHistory{
		{User: Signed{Data: "valid", Signature: "s1"}, Model: Signed{Data: "corrupted", Signature: "BLAH!"}},
		{User: Signed{Data: "valid", Signature: "s1"}, Model: Signed{Data: "valid", Signature: "s1"}},
	}

	// EXPECTATION: Return error on the specific corrupted block
	sv.On("Verify", mock.Anything, "BLAH!").Return(errors.New("invalid signature"))
	sv.On("Verify", mock.Anything, mock.Anything).Return(nil) // Allow others to pass until failure

	// WHEN
	err := history.Verify(sv)

	// THEN
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestChatHistory_AppendInPlace_Persistence(t *testing.T) {
	// GIVEN a null ChatHistory slice in the orchestrator
	var history ChatHistory

	// AND a signed User prompt and a signed Model response
	user := Signed{Data: "The ledger is AntiFragile."}
	model := Signed{Data: "I acknowledge the moral weight."}

	// WHEN we call AppendInPlace with a pointer to that history
	history.AppendInPlace(user, model)

	// THEN the history should contain exactly 1 node
	assert.Equal(t, 1, len(history))

	// AND the Model data in the first node should match our input
	assert.Equal(t, "I acknowledge the moral weight.", history[0].Model.Data)

	// AND the pointer address of the history header should reflect the growth
	// (This confirms we've avoided the 'Slice Header Shadowing' Greeble)
}

func TestChatHistory_Verify_ForensicIntegrity(t *testing.T) {
	// GIVEN a history with multiple signed turns
	sv := new(MockSignVerifier)
	history := ChatHistory{
		{User: Signed{Data: "P1", Signature: "s1"}, Model: Signed{Data: "R1", Signature: "s3"}},
		{User: Signed{Data: "P2", Signature: "s2"}, Model: Signed{Data: "R2", Signature: "s4"}},
	}

	// AND all signatures are physically valid
	sv.On("Verify", mock.Anything, mock.Anything).Return(nil)

	// WHEN we perform a full history verification
	err := history.Verify(sv)

	// THEN no error should be returned
	assert.NoError(t, err)

	// AND the SignVerifier should have been called for every block in the braid
	sv.AssertNumberOfCalls(t, "Verify", 4)
}
