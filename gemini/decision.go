package gemini

import (
	"slices"

	"github.com/weberr13/ProjectIolite/brain"
)

type GeminiDecision struct {
	cots    map[string][][]brain.Signed
	prompts map[string][]brain.Signed
	texts   map[string][]brain.Signed
}

func NewDecision(init brain.Response, sv brain.SignVerifier) (*GeminiDecision, error) {
	err := init.Verify(sv)
	if err != nil {
		return nil, err // response is corrupted/edited
	}
	d := &GeminiDecision{
		cots: map[string][][]brain.Signed{
			"gemini": {
				init.CoT(),
			},
		},
		prompts: map[string][]brain.Signed{
			"gemini": {
				init.Prompt(),
			},
		},
		texts: map[string][]brain.Signed{
			"gemini": {
				*init.Text(),
			},
		},
	}
	err = d.Sign(sv)
	return d, err
}

func (d *GeminiDecision) Sign(sv brain.SignVerifier) error {
	for k := range d.cots {
		if k == "gemini" {
			for i := range d.cots[k] {
				for j := range d.cots[k][i] {
					if d.cots[k][i][j].Signature == "" {
						sig, err := sv.Sign(d.cots[k][i][j].Data)
						if err != nil {
							return err
						}
						d.cots[k][i][j].Signature = sig
					}
				}
			}
		} else {
			for i := range d.cots[k] {
				for j := range d.cots[k][i] {
					if d.cots[k][i][j].Signature == "" {
						return ErrUnsigned
					}
				}
			}
		}
	}
	for k := range d.prompts {
		if k == "gemini" {
			for i := range d.prompts[k] {
				if d.prompts[k][i].Signature == "" {
					sig, err := sv.Sign(d.prompts[k][i].Data)
					if err != nil {
						return err
					}
					d.prompts[k][i].Signature = sig
				}
			}
		} else {
			for i := range d.prompts[k] {
				if d.prompts[k][i].Signature == "" {
					return ErrUnsigned
				}
			}
		}
	}
	for k := range d.texts {
		if k == "gemini" {
			for i := range d.texts[k] {
				if d.texts[k][i].Signature == "" {
					sig, err := sv.Sign(d.texts[k][i].Data)
					if err != nil {
						return err
					}
					d.texts[k][i].Signature = sig
				}
			}
		} else {
			for i := range d.texts[k] {
				if d.texts[k][i].Signature == "" {
					return ErrUnsigned
				}
			}
		}
	}
	return nil
}

func (d *GeminiDecision) Verify(sv brain.SignVerifier) error {
	for k := range d.cots {
		for i := range d.cots[k] {
			for j := range d.cots[k][i] {
				if d.cots[k][i][j].Signature == "" {
					return ErrUnsigned
				} else {
					err := sv.Verify(d.cots[k][i][j].Data, d.cots[k][i][j].Signature)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	for k := range d.prompts {
		for i := range d.prompts[k] {
			if d.prompts[k][i].Signature == "" {
				return ErrUnsigned
			} else {
				err := sv.Verify(d.prompts[k][i].Data, d.prompts[k][i].Signature)
				if err != nil {
					return err
				}
			}
		}
	}
	for k := range d.texts {
		for i := range d.texts[k] {
			if d.texts[k][i].Signature == "" {
				return ErrUnsigned
			} else {
				err := sv.Verify(d.texts[k][i].Data, d.texts[k][i].Signature)
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
	for k := range d.cots {
		for i := range d.cots[k] {
			m[k] = append(m[k], slices.Clone(d.cots[k][i]))
		}
	}
	return m
}

func (d *GeminiDecision) Prompts() map[string][]brain.Signed {
	m := make(map[string][]brain.Signed)
	for k := range d.prompts {
		m[k] = slices.Clone(d.prompts[k])
	}

	return m
}

func (d *GeminiDecision) Texts() map[string][]brain.Signed {
	m := make(map[string][]brain.Signed)
	for k := range d.texts {
		m[k] = slices.Clone(d.texts[k])
	}

	return m
}

func (d *GeminiDecision) Add(cot []brain.Signed, text brain.Signed, sv brain.SignVerifier) error {
	d.cots["gemini"] = append(d.cots["gemini"], cot)
	d.texts["gemini"] = append(d.texts["gemini"], text)
	err := d.Sign(sv) // sign all unsigned things
	if err != nil {
		return err
	}
	return d.Verify(sv)
}
