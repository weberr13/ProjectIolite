package brain

import "slices"

type BaseDecision struct {
	ChainOfThoughts map[string][][]Signed
	AllPrompts      map[string][]Signed
	AllTexts        map[string][]Signed
	Source          string
	e               error
}

func (e *BaseDecision) IsError() error {
	return e.e
}

func (e *BaseDecision) SetError(err error) {
	e.e = err
}

func NewBaseDecision(source string, init Response, sv SignVerifier) (*BaseDecision, error) {
	b := &BaseDecision{
		Source: source,
		ChainOfThoughts: map[string][][]Signed{
			init.Source(): {
				init.CoT(),
			},
		},
		AllPrompts: map[string][]Signed{
			init.Source(): {
				init.Prompt(),
			},
		},
		AllTexts: map[string][]Signed{},
	}
	tx := init.Text()
	if tx != nil {
		b.AllTexts[init.Source()] = []Signed{*tx}
	}
	return b, b.Sign(sv)
}

func (d *BaseDecision) Sign(sv SignVerifier) error {
	for k := range d.ChainOfThoughts {
		if k == d.Source {
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
						return ErrUnsigned
					}
				}
			}
		}
	}
	for k := range d.AllPrompts {
		if k == d.Source {
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
					return ErrUnsigned
				}
			}
		}
	}
	for k := range d.AllTexts {
		if k == d.Source {
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
					return ErrUnsigned
				}
			}
		}
	}
	return nil
}

func (d *BaseDecision) Verify(sv SignVerifier) error {
	for k := range d.ChainOfThoughts {
		for i := range d.ChainOfThoughts[k] {
			for j := range d.ChainOfThoughts[k][i] {
				if d.ChainOfThoughts[k][i][j].Signature == "" {
					return ErrUnsigned
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
				return ErrUnsigned
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
				return ErrUnsigned
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

func (d *BaseDecision) Cots() map[string][][]Signed {
	m := make(map[string][][]Signed)
	// deep copy
	for k := range d.ChainOfThoughts {
		for i := range d.ChainOfThoughts[k] {
			m[k] = append(m[k], slices.Clone(d.ChainOfThoughts[k][i]))
		}
	}
	return m
}

func (d *BaseDecision) Prompts() map[string][]Signed {
	m := make(map[string][]Signed)
	for k := range d.AllPrompts {
		m[k] = slices.Clone(d.AllPrompts[k])
	}

	return m
}

func (d *BaseDecision) Texts() map[string][]Signed {
	m := make(map[string][]Signed)
	for k := range d.AllTexts {
		m[k] = slices.Clone(d.AllTexts[k])
	}

	return m
}

func (d *BaseDecision) Add(sourceID string, cot []Signed, text Signed, sv SignVerifier) error {
	// TODO: each "unit" of the chain of thought, chain, should have a signature too!
	d.ChainOfThoughts[sourceID] = append(d.ChainOfThoughts[sourceID], cot)
	last := len(d.AllTexts[sourceID]) - 1
	if last >= 0 {
		// Join the new text to the chain before signing
		text.PrevSignature = d.AllTexts[sourceID][last].Signature
	}
	d.AllTexts[sourceID] = append(d.AllTexts[sourceID], text)
	err := d.Sign(sv) // sign all unsigned things
	if err != nil {
		return err
	}
	return d.Verify(sv)
}
