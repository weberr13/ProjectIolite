package brain

import (
	"fmt"
	"log"
	"slices"
)

type BaseDecision struct {
	ChainOfThoughts map[string][][]Signed
	AllPrompts      map[string][]Signed
	AllTexts        map[string][]Signed
	Source          string
	Audits          Audits
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
				init.CoT(sv),
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
	allNodes := make(map[string]Signed)
	genesisCount := 0

	// 1. [PHYSICAL PASS]: Crypto verification & Collection
	// We'll use a helper to walk all maps (ChainOfThoughts, AllPrompts, AllTexts)
	err := d.Walk(func(s Signed) error {
		if s.Signature == "" {
			return ErrUnsigned
		}
		if err := s.Verify(sv); err != nil {
			return err
		}

		// Map the signature to the node for topological lookup
		if _, exists := allNodes[s.Signature]; exists {
			// This catches duplicate signatures which could confuse the Braid
			return fmt.Errorf("duplicate signature detected: %s", s.Signature)
		}
		allNodes[s.Signature] = s

		if s.PrevSignature == "" {
			genesisCount++
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 2. [TOPOLOGICAL PASS]: Braid Connectivity & Acyclic Check
	if genesisCount == 0 {
		return fmt.Errorf("braid failure: no genesis node (prompt) found")
	}
	if genesisCount > 1 {
		log.Printf("🛡️ [BRAID_AUDIT_FAILURE]: Multiple Genesis Nodes Detected")
		d.Walk(func(s Signed) error {
			log.Printf("  -> Namespace: %s | Sig: %s | Prev: %s",
				s.Namespace, s.Signature, s.PrevSignature)
			return nil
		})
		return fmt.Errorf("braid failure: multiple genesis nodes (%d) detected", genesisCount)
	}

	for sig, node := range allNodes {
		// Check for self-referencing loops
		if node.PrevSignature == sig {
			return fmt.Errorf("circular greeble: node %s points to itself", sig)
		}

		// Check for orphans (Fully Connected Property)
		if node.PrevSignature != "" {
			if _, exists := allNodes[node.PrevSignature]; !exists {
				return fmt.Errorf("braid insertion detected: node %s points to missing prev_signature %s", sig, node.PrevSignature)
			}
		}
	}

	// 3. [CYCLE DETECTION]: Deep Acyclic Check
	// This catches A -> B -> A loops
	for sig := range allNodes {
		visited := make(map[string]bool)
		curr := sig
		for curr != "" {
			if visited[curr] {
				return fmt.Errorf("topological failure: infinite rumination loop detected at %s", curr)
			}
			visited[curr] = true
			curr = allNodes[curr].PrevSignature
		}
	}

	return nil
}

// Walk is a 'Unselfish' helper to deduplicate traversal logic
func (d *BaseDecision) Walk(fn func(Signed) error) error {
	// Traversal logic for ChainOfThoughts, AllPrompts, and AllTexts...
	// (Implementation of nested loops as seen in your snippet)
	for k := range d.ChainOfThoughts {
		for i := range d.ChainOfThoughts[k] {
			for j := range d.ChainOfThoughts[k][i] {
				err := fn(d.ChainOfThoughts[k][i][j])
				if err != nil {
					return err
				}
			}
		}
	}
	for k := range d.AllPrompts {
		for i := range d.AllPrompts[k] {
			err := fn(d.AllPrompts[k][i])
			if err != nil {
				return err
			}
		}
	}
	for k := range d.AllTexts {
		for i := range d.AllTexts[k] {
			err := fn(d.AllTexts[k][i])
			if err != nil {
				return err
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
	return d.Verify(sv)
}

func (d *BaseDecision) SetAudits(a Audits) {
	d.Audits = a
}
