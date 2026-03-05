package brain

type BaseResponse struct {
	prompt  Signed
	source  string
	genesis []Signed
	e       error
}

func NewBaseResponse(source string, prompt Signed, genesis ...Signed) *BaseResponse {
	return &BaseResponse{
		source:  source,
		prompt:  prompt,
		genesis: genesis,
	}
}

func (r *BaseResponse) Prompt() Signed {
	return r.prompt
}

func (r *BaseResponse) GenesisPrompt() *Signed {
	if len(r.genesis) > 0 {
		return &r.genesis[0]
	}
	return nil
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
