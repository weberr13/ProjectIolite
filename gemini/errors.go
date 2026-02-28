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
		brain.NewUnsigned(e.e.Error(), "cot"),
	}
}

func (e *GeminiError) Verify(sv brain.SignVerifier) error {
	if e.sig == "" {
		return nil // this is ok, internal program state
	}
	return sv.Verify(e.e.Error(), e.sig)
}

func (e *GeminiError) Text() *brain.Signed {
	s := brain.NewUnsigned(e.e.Error(), "text")
	return &s
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
	return brain.NewUnsigned(e.input, "prompt")
}

func (e *GeminiError) Source() string {
	return "gemini"
}

// Error implements the error interface.
// We use the ":" separator which is standard for wrapped errors.
func (e *GeminiError) Error() string {
	return fmt.Sprintf("gemini: %v (input: %q)", e.e, e.input)
}

// Unwrap allows errors.Is and errors.As to access the internal error.
// This is the "Magic Key" for the errors package.
func (e *GeminiError) Unwrap() error {
	return e.e
}

func (e *GeminiError) IsError() error {
	return e.e
}
