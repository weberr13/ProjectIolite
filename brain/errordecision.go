package brain

import (
	"fmt"
)

type ErrorDecision struct {
	E error
}

func (ErrorDecision) Cots() map[string][][]Signed {
	return nil
}

func (ErrorDecision) Prompts() map[string][]Signed {
	return nil
}

func (e *ErrorDecision) Texts() map[string][]Signed {
	return map[string][]Signed{
		"system": {
			{Data: e.E.Error()},
		},
	}
}

func (ErrorDecision) Sign(SignVerifier) error {
	return nil
}

func (ErrorDecision) Verify(SignVerifier) error {
	return nil
}

func (e *ErrorDecision) Add(cot []Signed, text Signed, sv SignVerifier) error {
	e.E = fmt.Errorf("%s: %w", text.Data, e.E)
	return nil
}
