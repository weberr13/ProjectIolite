package brain

import (
	"context"
	"errors"
	"fmt"
	"log"
)

var (
	ErrDuplicateSignature   = errors.New("dupliicate signature detected")
	ErrNoGenesisNode        = errors.New("braid failure: no genesis node found")
	ErrDuplicateGenesisNode = errors.New("braid failure: more than one genesis node found")
	ErrCircular             = errors.New("circular dependency")
	ErrOrphan               = errors.New("orphaned branch found")
	ErrCycle                = errors.New("topological failure cyclic reference")
)

type SignedContainer interface {
	Blocks() []*Signed
	GetPublicKey() string
}

func VerifyBraid(ctx context.Context, sv SignVerifier, data SignedContainer) error {
	allNodes := make(map[string]*Signed)
	genesisCount := 0

	err := Walk(data.Blocks(), func(s *Signed) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if s.Signature == "" {
			log.Printf("UNSIGNED: %#v", s)
			return ErrUnsigned
		}
		if data.GetPublicKey() == "" {
			if err := s.Verify(sv); err != nil {
				log.Printf("failed on %#v with built in public key", s)
				return err
			}
		} else {
			if err := s.Verify(sv, data.GetPublicKey()); err != nil {
				log.Printf("failed on %#v with public key %s", s, data.GetPublicKey())
				return err
			}
		}

		// Map the signature to the node for topological lookup
		if _, exists := allNodes[s.Signature]; exists {
			// This catches duplicate signatures which could confuse the Braid
			return fmt.Errorf("%w: %v == %v %#v", ErrDuplicateSignature, s, allNodes[s.Signature], data)
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

	if genesisCount == 0 {
		return ErrNoGenesisNode
	}
	if genesisCount > 1 {
		log.Printf("🛡️ [BRAID_AUDIT_FAILURE]: Multiple Genesis Nodes Detected")
		Walk(data.Blocks(), func(s *Signed) error {
			log.Printf("  -> Namespace: %s | Sig: %s | Prev: %s",
				s.Namespace, s.Signature, s.PrevSignature)
			return nil
		})
		return fmt.Errorf("%w, (%d) detected", ErrDuplicateGenesisNode, genesisCount)
	}

	for sig, node := range allNodes {
		// Check for self-referencing loops
		if node.PrevSignature == sig {
			return fmt.Errorf("%w: node %s points to itself", ErrCircular, sig)
		}

		// Check for orphans (Fully Connected Property)
		if node.PrevSignature != "" {
			if _, exists := allNodes[node.PrevSignature]; !exists {
				return fmt.Errorf("%w: node %s points to missing prev_signature %s", ErrOrphan, sig, node.PrevSignature)
			}
		}
	}

	// This catches A -> B -> A loops
	for sig := range allNodes {
		visited := make(map[string]bool)
		curr := sig
		for curr != "" {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if visited[curr] {
				return fmt.Errorf("%w: detected at %s", ErrCycle, curr)
			}
			visited[curr] = true
			curr = allNodes[curr].PrevSignature
		}
	}

	return nil
}

func Walk(blocks []*Signed, fn func(*Signed) error) error {
	for i := range blocks {
		err := fn(blocks[i])
		if err != nil {
			return err
		}
	}
	return nil
}
