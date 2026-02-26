package gemini

import (
	"slices"

	"github.com/weberr13/ProjectIolite/brain"
)

type GeminiDecision struct {
	ChainOfThoughts map[string][][]brain.Signed
	AllPrompts      map[string][]brain.Signed
	AllTexts        map[string][]brain.Signed
}

func NewDecision(init brain.Response, sv brain.SignVerifier) (*GeminiDecision, error) {
	err := init.Verify(sv)
	if err != nil {
		return nil, err // response is corrupted/edited
	}
	d := &GeminiDecision{
		ChainOfThoughts: map[string][][]brain.Signed{
			"gemini": {
				init.CoT(),
			},
		},
		AllPrompts: map[string][]brain.Signed{
			"gemini": {
				init.Prompt(),
			},
		},
		AllTexts: map[string][]brain.Signed{
			"gemini": {
				*init.Text(),
			},
		},
	}
	err = d.Sign(sv)
	return d, err
}

func (d *GeminiDecision) Sign(sv brain.SignVerifier) error {
	for k := range d.ChainOfThoughts {
		if k == "gemini" {
			for i := range d.ChainOfThoughts[k] {
				for j := range d.ChainOfThoughts[k][i] {
					if d.ChainOfThoughts[k][i][j].Signature == "" {
						sig, err := sv.Sign(d.ChainOfThoughts[k][i][j].Data)
						if err != nil {
							return err
						}
						d.ChainOfThoughts[k][i][j].Signature = sig
					}
				}
			}
		} else {
			for i := range d.ChainOfThoughts[k] {
				for j := range d.ChainOfThoughts[k][i] {
					if d.ChainOfThoughts[k][i][j].Signature == "" {
						return ErrUnsigned
					}
				}
			}
		}
	}
	for k := range d.AllPrompts {
		if k == "gemini" {
			for i := range d.AllPrompts[k] {
				if d.AllPrompts[k][i].Signature == "" {
					sig, err := sv.Sign(d.AllPrompts[k][i].Data)
					if err != nil {
						return err
					}
					d.AllPrompts[k][i].Signature = sig
				}
			}
		} else {
			for i := range d.AllPrompts[k] {
				if d.AllPrompts[k][i].Signature == "" {
					return ErrUnsigned
				}
			}
		}
	}
	for k := range d.AllTexts {
		if k == "gemini" {
			for i := range d.AllTexts[k] {
				if d.AllTexts[k][i].Signature == "" {
					sig, err := sv.Sign(d.AllTexts[k][i].Data)
					if err != nil {
						return err
					}
					d.AllTexts[k][i].Signature = sig
				}
			}
		} else {
			for i := range d.AllTexts[k] {
				if d.AllTexts[k][i].Signature == "" {
					return ErrUnsigned
				}
			}
		}
	}
	return nil
}

func (d *GeminiDecision) Verify(sv brain.SignVerifier) error {
	for k := range d.ChainOfThoughts {
		for i := range d.ChainOfThoughts[k] {
			for j := range d.ChainOfThoughts[k][i] {
				if d.ChainOfThoughts[k][i][j].Signature == "" {
					return ErrUnsigned
				} else {
					err := sv.Verify(d.ChainOfThoughts[k][i][j].Data, d.ChainOfThoughts[k][i][j].Signature)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	for k := range d.AllPrompts {
		for i := range d.AllPrompts[k] {
			if d.AllPrompts[k][i].Signature == "" {
				return ErrUnsigned
			} else {
				err := sv.Verify(d.AllPrompts[k][i].Data, d.AllPrompts[k][i].Signature)
				if err != nil {
					return err
				}
			}
		}
	}
	for k := range d.AllTexts {
		for i := range d.AllTexts[k] {
			if d.AllTexts[k][i].Signature == "" {
				return ErrUnsigned
			} else {
				err := sv.Verify(d.AllTexts[k][i].Data, d.AllTexts[k][i].Signature)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d *GeminiDecision) Cots() map[string][][]brain.Signed {
	m := make(map[string][][]brain.Signed)
	// deep copy
	for k := range d.ChainOfThoughts {
		for i := range d.ChainOfThoughts[k] {
			m[k] = append(m[k], slices.Clone(d.ChainOfThoughts[k][i]))
		}
	}
	return m
}

func (d *GeminiDecision) Prompts() map[string][]brain.Signed {
	m := make(map[string][]brain.Signed)
	for k := range d.AllPrompts {
		m[k] = slices.Clone(d.AllPrompts[k])
	}

	return m
}

func (d *GeminiDecision) Texts() map[string][]brain.Signed {
	m := make(map[string][]brain.Signed)
	for k := range d.AllTexts {
		m[k] = slices.Clone(d.AllTexts[k])
	}

	return m
}

func (d *GeminiDecision) Add(cot []brain.Signed, text brain.Signed, sv brain.SignVerifier) error {
	d.ChainOfThoughts["gemini"] = append(d.ChainOfThoughts["gemini"], cot)
	d.AllTexts["gemini"] = append(d.AllTexts["gemini"], text)
	err := d.Sign(sv) // sign all unsigned things
	if err != nil {
		return err
	}
	return d.Verify(sv)
}
