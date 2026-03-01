package claude

import (
	"log"

	"github.com/weberr13/ProjectIolite/brain"
)

type ClaudeDecision struct {
	*brain.BaseDecision
}

func NewDecision(init brain.Response, sv brain.SignVerifier) (*ClaudeDecision, error) {
	err := init.Verify(sv)
	if err != nil {
		log.Printf("failed to verify response: %s", err)
		return nil, err // response is corrupted/edited
	}
	b, err := brain.NewBaseDecision("claude", init, sv)
	d := &ClaudeDecision{
		BaseDecision: b,
	}
	return d, err
}
