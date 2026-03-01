package brain

type BaseResponse struct {
	prompt Signed
	source string
	e      error
}

func NewBaseResponse(source string, prompt Signed) *BaseResponse {
	return &BaseResponse{
		source: source,
		prompt: prompt,
	}
}

func (r *BaseResponse) Prompt() Signed {
	return r.prompt
}

func (r *BaseResponse) Source() string {
	return r.source
}

func (r *BaseResponse) SignPrompt(sv SignVerifier) error {
	if r.prompt.Signature == "" {
		err := r.prompt.Sign(sv)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *BaseResponse) IsError() error {
	return r.e
}

func (r *BaseResponse) SetError(err error) {
	r.e = err
}
