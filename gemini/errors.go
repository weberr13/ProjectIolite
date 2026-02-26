package gemini

import (
	"fmt"

	"github.com/weberr13/ProjectIolite/brain"
)

type GeminiError struct {
	e     error
	input string
	sig   string
}

func (e *GeminiError) Sign(sv brain.SignVerifier) error {
	sig, err := sv.Sign(e.e.Error())
	if err != nil {
		return err
	}
	e.sig = sig
	return err
}

func (e *GeminiError) CoT() []brain.Signed {
	return []brain.Signed{
		{Data: e.e.Error()},
	}
}

func (e *GeminiError) Verify(sv brain.SignVerifier) error {
	if e.sig == "" {
		return nil // this is ok, internal program state
	}
	return sv.Verify(e.e.Error(), e.sig)
}

func (e *GeminiError) Text() *brain.Signed {
	return &brain.Signed{Data: e.e.Error()}
}

func (e *GeminiError) Describe(sv brain.SignVerifier) string {
	if e.sig == "" {
		sig, err := sv.Sign(e.e.Error())
		if err == nil {
			e.sig = sig
		}
	}
	return fmt.Sprintf("a critical error occurred and processing could not proceed: %s", e.e.Error())
}

func (e *GeminiError) Prompt() brain.Signed {
	return brain.Signed{Data: e.input}
}
