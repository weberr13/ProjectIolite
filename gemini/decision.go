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
						err := d.ChainOfThoughts[k][i][j].Sign(sv)
						if err != nil {
							return err
						}
					}
				}
			}
		} else {
			for i := range d.ChainOfThoughts[k] {
				for j := range d.ChainOfThoughts[k][i] {
					if d.ChainOfThoughts[k][i][j].Signature == "" {
						return brain.ErrUnsigned
					}
				}
			}
		}
	}
	for k := range d.AllPrompts {
		if k == "gemini" {
			for i := range d.AllPrompts[k] {
				if d.AllPrompts[k][i].Signature == "" {
					err := d.AllPrompts[k][i].Sign(sv)
					if err != nil {
						return err
					}
				}
			}
		} else {
			for i := range d.AllPrompts[k] {
				if d.AllPrompts[k][i].Signature == "" {
					return brain.ErrUnsigned
				}
			}
		}
	}
	for k := range d.AllTexts {
		if k == "gemini" {
			for i := range d.AllTexts[k] {
				if d.AllTexts[k][i].Signature == "" {
					err := d.AllTexts[k][i].Sign(sv)
					if err != nil {
						return err
					}
				}
			}
		} else {
			for i := range d.AllTexts[k] {
				if d.AllTexts[k][i].Signature == "" {
					return brain.ErrUnsigned
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
					return brain.ErrUnsigned
				} else {
					err := d.ChainOfThoughts[k][i][j].Verify(sv)
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
				return brain.ErrUnsigned
			} else {
				err := d.AllPrompts[k][i].Verify(sv)
				if err != nil {
					return err
				}
			}
		}
	}
	for k := range d.AllTexts {
		for i := range d.AllTexts[k] {
			if d.AllTexts[k][i].Signature == "" {
				return brain.ErrUnsigned
			} else {
				err := d.AllTexts[k][i].Verify(sv)
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
	// TODO: each "unit" of the chain of thought, chain, should have a signature too!
	d.ChainOfThoughts["gemini"] = append(d.ChainOfThoughts["gemini"], cot)
	last := len(d.AllTexts["gemini"]) - 1
	if last >= 0 {
		// Join the new text to the chain before signing
		text.PrevSignature = d.AllTexts["gemini"][last].Signature
	}
	d.AllTexts["gemini"] = append(d.AllTexts["gemini"], text)
	err := d.Sign(sv) // sign all unsigned things
	if err != nil {
		return err
	}
	return d.Verify(sv)
}
