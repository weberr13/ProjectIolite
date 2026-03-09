package brain

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"
)

type BaseDecision struct {
	ChainOfThoughts  map[string][][]Signed
	AllPrompts       map[string][]Signed
	AllTexts         map[string][]Signed
	AllToolRequests  map[string][]Signed
	AllToolResponses map[string][]Signed
	Source           string
	Audits           Audits
	e                error
	PublicKey        string
}

func (e *BaseDecision) GetPublicKey() string {
	return e.PublicKey
}

func (e *BaseDecision) Compose(sv SignVerifier, d Decision) error {
	otherCoT := d.Cots()
	for k := range otherCoT {
		e.ChainOfThoughts[k] = append(e.ChainOfThoughts[k], otherCoT[k]...)
	}
	log.Printf("merging prompts\n")
	e.AllPrompts = mergeSignedSlices(e.AllPrompts, d.Prompts())
	log.Printf("merging text\n")
	e.AllTexts = mergeSignedSlices(e.AllTexts, d.Texts())
	log.Printf("merging tool requests\n")
	e.AllToolRequests = mergeSignedSlices(e.AllToolRequests, d.ToolRequests())
	log.Printf("merging tool responses\n")
	e.AllToolResponses = mergeSignedSlices(e.AllToolResponses, d.ToolResponses())
	e.Audits = append(e.Audits, d.GetAudits()...)
	log.Printf("test verify: %s", e.Verify(sv))
	return nil
}

// Internal helper to merge slices based on signature parity
func mergeSignedSlices(existing, incoming map[string][]Signed) map[string][]Signed {
	for k, exBlocks := range existing {
		for _, b := range exBlocks {
			log.Printf("%s existing: %#v\n", k, b)
		}
	}
	for k, inBlocks := range incoming {
		for _, b := range inBlocks {
			log.Printf("%s incoming: %#v\n", k, b)
		}
	}
	for k, inBlocks := range incoming {
		// Create a map of existing signatures for this namespace
		seen := make(map[string]bool)
		for _, b := range existing[k] {
			seen[b.Signature] = true
		}

		for _, b := range inBlocks {
			if !seen[b.Signature] {
				existing[k] = append(existing[k], b)
			} else {
				log.Printf("got a block we've already seen?? %#v", b)
			}
		}
	}
	return existing
}

func (e *BaseDecision) IsError() error {
	return e.e
}

func (e *BaseDecision) SetError(err error) {
	e.e = err
}

func NewBaseDecision(source string, init Response, sv SignVerifier) (*BaseDecision, error) {
	if source == "mock" {
		return nil, errors.New("no mock models allowed")
	}
	allPrompts := map[string][]Signed{}
	genesis := init.GenesisPrompt()
	if genesis != nil {
		allPrompts[init.Source()] = []Signed{*genesis}
	}
	allPrompts[init.Source()] = append(allPrompts[init.Source()], init.Prompt())
	b := &BaseDecision{
		Source: source,
		ChainOfThoughts: map[string][][]Signed{
			init.Source(): {
				init.CoT(sv),
			},
		},
		AllPrompts:       allPrompts,
		AllTexts:         map[string][]Signed{},
		AllToolRequests:  map[string][]Signed{},
		AllToolResponses: map[string][]Signed{},
		PublicKey:        sv.ExportPublicKey(),
	}

	tx := init.Text()
	if tx != nil {
		b.AllTexts[init.Source()] = []Signed{*tx}
	}
	return b, b.Sign(sv)
}

func (d *BaseDecision) Sign(sv SignVerifier) error {
	for k := range d.ChainOfThoughts {
		if k == d.Source || d.Source == "mock" {
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
		if k == d.Source || d.Source == "mock" {
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
		if k == d.Source || d.Source == "mock" {
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
	for k := range d.AllToolRequests {
		if k == d.Source || d.Source == "mock" {
			for i := range d.AllToolRequests[k] {
				if d.AllToolRequests[k][i].Signature == "" {
					err := d.AllToolRequests[k][i].Sign(sv)
					if err != nil {
						return err
					}
				}
			}
		} else {
			for i := range d.AllToolRequests[k] {
				if d.AllToolRequests[k][i].Signature == "" {
					return ErrUnsigned
				}
			}
		}
	}
	for k := range d.AllToolResponses {
		if k == d.Source || d.Source == "mock" {
			for i := range d.AllToolResponses[k] {
				if d.AllToolResponses[k][i].Signature == "" {
					err := d.AllToolResponses[k][i].Sign(sv)
					if err != nil {
						return err
					}
				}
			}
		} else {
			for i := range d.AllToolResponses[k] {
				if d.AllToolResponses[k][i].Signature == "" {
					return ErrUnsigned
				}
			}
		}
	}
	return nil
}

func (d *BaseDecision) Blocks() []*Signed {
	bs := []*Signed{}
	for k := range d.ChainOfThoughts {
		for i := range d.ChainOfThoughts[k] {
			for j := range d.ChainOfThoughts[k][i] {
				bs = append(bs, &d.ChainOfThoughts[k][i][j])
			}
		}
	}
	for k := range d.AllPrompts {
		for i := range d.AllPrompts[k] {
			bs = append(bs, &d.AllPrompts[k][i])
		}
	}
	for k := range d.AllTexts {
		for i := range d.AllTexts[k] {
			bs = append(bs, &d.AllTexts[k][i])
		}
	}
	for k := range d.AllToolRequests {
		for i := range d.AllToolRequests[k] {
			bs = append(bs, &d.AllToolRequests[k][i])
		}
	}
	for k := range d.AllToolResponses {
		for i := range d.AllToolResponses[k] {
			bs = append(bs, &d.AllToolResponses[k][i])
		}
	}
	return bs
}

func (d *BaseDecision) Verify(sv SignVerifier) error {
	// TODO: inject the app context instead of context.Background
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return VerifyBraid(ctx, sv, d)
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

func (d *BaseDecision) ToolRequests() map[string][]Signed {
	m := make(map[string][]Signed)
	for k := range d.AllToolRequests {
		m[k] = slices.Clone(d.AllToolRequests[k])
	}

	return m
}

func (d *BaseDecision) ToolResponses() map[string][]Signed {
	m := make(map[string][]Signed)
	for k := range d.AllToolResponses {
		m[k] = slices.Clone(d.AllToolResponses[k])
	}

	return m
}

func (d *BaseDecision) Add(sourceID string, cot []Signed, text Signed, sv SignVerifier, tools ...[]Signed) error {
	// 1. [FORENSIC ANCHOR]: Find the tail of the current Braid for this source
	var anchor string

	// Look at previous texts first
	if lastIdx := len(d.AllTexts[sourceID]) - 1; lastIdx >= 0 {
		anchor = d.AllTexts[sourceID][lastIdx].Signature
	} else if lastPromptIdx := len(d.AllPrompts[sourceID]) - 1; lastPromptIdx >= 0 {
		// Fallback to the Genesis Prompt if this is the first response
		anchor = d.AllPrompts[sourceID][lastPromptIdx].Signature
	}

	// 2. [BRAID STITCHING]: Set the PrevSignature BEFORE the Sign/Verify pass
	if text.PrevSignature == "" && anchor != "" {
		text.PrevSignature = anchor
	}

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
	switch len(tools) {
	case 1:
		d.AllToolRequests[sourceID] = append(d.AllToolRequests[sourceID], tools[0]...)
		err := d.Sign(sv) // sign all unsigned things
		if err != nil {
			return err
		}
	case 2:
		d.AllToolRequests[sourceID] = append(d.AllToolRequests[sourceID], tools[0]...)
		err := d.Sign(sv) // sign all unsigned things
		if err != nil {
			return err
		}
		d.AllToolResponses[sourceID] = append(d.AllToolResponses[sourceID], tools[1]...)
		err = d.Sign(sv) // sign all unsigned things
		if err != nil {
			return err
		}
	}
	return d.Verify(sv)
}

func (d *BaseDecision) SetAudits(a Audits) {
	d.Audits = append(d.Audits, a...)
}

func (d *BaseDecision) GetAudits() Audits {
	return d.Audits
}

func (d BaseDecision) GenesisPrompt() Signed {
	ps := d.Prompts()
	for k := range ps {
		if len(ps[k]) == 0 { // only one model may 'own' the prompts
			continue
		}
		for _, p := range ps[k] {
			if p.PrevSignature == "" {
				return p
			}
		}
	}
	return Signed{}
}
