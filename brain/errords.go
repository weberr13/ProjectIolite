package brain

import (
	"fmt"
)

type ErrorResponse struct {
	E error
}

func (ErrorResponse) CoT(_ SignVerifier) []Signed {
	return nil
}

func (e *ErrorResponse) Text() *Signed {
	return &Signed{Data: e.E.Error()}
}

func (e *ErrorResponse) Prompt() Signed {
	return Signed{Data: e.E.Error()}
}

func (e *ErrorResponse) Describe(SignVerifier) string {
	return e.E.Error()
}

func (e *ErrorResponse) Sign(SignVerifier) error {
	return e.E
}

func (e *ErrorResponse) Verify(SignVerifier) error {
	return e.E
}

func (e *ErrorResponse) Source() string {
	return e.E.Error()
}

func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("system error: %v", e.E)
}

func (e *ErrorResponse) Unwrap() error {
	return e.E
}

func (e *ErrorResponse) IsError() error {
	return e.E
}

func (e *ErrorResponse) SetError(err error) {
	e.E = err
}

type ErrorDecision struct {
	E error
}

func (ErrorDecision) Cots() map[string][][]Signed {
	return nil
}

func (ErrorDecision) Prompts() map[string][]Signed {
	return nil
}

func (e *ErrorDecision) Error() string {
	return fmt.Sprintf("system error: %v", e.E)
}

func (e *ErrorDecision) IsError() error {
	return e.E
}

func (e *ErrorDecision) SetError(err error) {
	e.E = err
}

func (e *ErrorDecision) Unwrap() error {
	return e.E
}

func (ErrorDecision) SetAuditss(Audits) {
}

func (e *ErrorDecision) Texts() map[string][]Signed {
	return map[string][]Signed{
		"system": {
			NewUnsigned(fmt.Sprintf("CRITICAL_FAILURE: %v", e.E), TypeText),
		},
	}
}

func (ErrorDecision) Sign(SignVerifier) error {
	return nil
}

func (ErrorDecision) Verify(SignVerifier) error {
	return nil
}

func (e *ErrorDecision) Add(_ string, cot []Signed, text Signed, sv SignVerifier) error {
	e.E = fmt.Errorf("%s: %w", text.Data, e.E)
	return nil
}

func (ErrorDecision) SetAudits(Audits) {
}
