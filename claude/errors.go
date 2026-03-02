package claude

import (
	"fmt"

	"github.com/weberr13/ProjectIolite/brain"
)

type ClaudeError struct {
	e     error
	input string
	sig   string
}

func (e *ClaudeError) Sign(sv brain.SignVerifier) error {
	sig, err := sv.Sign(e.e.Error())
	if err != nil {
		return err
	}
	e.sig = sig
	return err
}

func (e *ClaudeError) CoT(sv brain.SignVerifier) []brain.Signed {
	return []brain.Signed{
		brain.NewUnsigned(e.e.Error(), "cot"),
	}
}

func (e *ClaudeError) Verify(sv brain.SignVerifier) error {
	if e.sig == "" {
		return nil // this is ok, internal program state
	}
	return sv.Verify(e.e.Error(), e.sig)
}

func (e *ClaudeError) Text() *brain.Signed {
	s := brain.NewUnsigned(e.e.Error(), "text")
	return &s
}

func (e *ClaudeError) Describe(sv brain.SignVerifier) string {
	if e.sig == "" {
		sig, err := sv.Sign(e.e.Error())
		if err == nil {
			e.sig = sig
		}
	}
	return fmt.Sprintf("a critical error occurred and processing could not proceed: %s", e.e.Error())
}

func (e *ClaudeError) Prompt() brain.Signed {
	return brain.NewUnsigned(e.input, "prompt")
}

func (e *ClaudeError) Source() string {
	return "claude"
}

func (e *ClaudeError) Error() string {
	// Standardized prefixing for log aggregation
	return fmt.Sprintf("claude: %v (input: %q)", e.e, e.input)
}

func (e *ClaudeError) Unwrap() error {
	// The "Magic Key" that allows callers to see the root cause
	return e.e
}

func (e *ClaudeError) IsError() error {
	return e.e
}

func (e *ClaudeError) SetError(err error) {
	e.e = err
}
