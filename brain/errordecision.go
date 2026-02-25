package brain

type ErrorDecision struct {
	E error
}

func (ErrorDecision) Cots() map[string][][]Signed {
	return nil
}

func (ErrorDecision) Prompts() map[string][]Signed {
	return nil
}

func (e ErrorDecision) Texts() map[string][]Signed {
	return map[string][]Signed{
		"system": {
			{Data: e.E.Error()},
		},
	}
}
