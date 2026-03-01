package gemini

import (
	"github.com/weberr13/ProjectIolite/brain"
)

type GeminiDecision struct {
	*brain.BaseDecision
}

func NewDecision(init brain.Response, sv brain.SignVerifier) (*GeminiDecision, error) {
	err := init.Verify(sv)
	if err != nil {
		return nil, err // response is corrupted/edited
	}
	b, err := brain.NewBaseDecision("gemini", init, sv)
	d := &GeminiDecision{
		BaseDecision: b,
	}
	return d, err
}
