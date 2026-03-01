package brain

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"
)

type DecisionParser struct{}

type HasTexts interface {
	Texts() map[string][]Signed // map of model -> turn responses
	// Prompts() map[string][]Signed // the sequence of prompts given to each model
}

func (p *DecisionParser) IsApproved(sv SignVerifier, dec HasTexts) (bool, error) {
	texts := dec.Texts()

	// 1. Build the Braid Map: Signature -> Block
	// And track which Signatures are 'pointed to' by others.
	allBlocks := make(map[string]Signed)
	hasParent := make(map[string]bool)

	for _, turns := range texts {
		for _, turn := range turns {
			allBlocks[turn.Signature] = turn
			hasParent[turn.PrevSignature] = true
		}
	}

	// 2. Identify the 'Terminal Node'
	// The winner is the block that has an opinion AND is not a 'PrevSignature' for anyone else.
	var terminal Signed
	foundTerminal := false

	for sig, block := range allBlocks {
		if !hasParent[sig] {
			// This is a leaf node (nothing points to it)
			terminal = block
			foundTerminal = true
			break
		}
	}

	if !foundTerminal {
		return false, ErrNoConsensus
	}

	// 3. Walk BACKWARDS from the Terminal Node to verify the chain and find the latest opinion
	current := terminal
	foundOpinion := false
	verdict := false

	for {
		// Mandatory Integrity Check
		if err := current.Verify(sv); err != nil {
			return false, err
		}

		// Extract Opinion if we haven't found the 'Latest' one yet
		// Since we are walking BACKWARDS, the first opinion we find
		// is technically the 'Last' one in the causal chain.
		if !foundOpinion {
			payload := current.Data
			if decoded, err := base64.StdEncoding.DecodeString(current.Data); err == nil {
				payload = string(decoded)
			}

			jsons := parseForJsonBlocks(payload)
			for i := len(jsons) - 1; i >= 0; i-- {
				// We found a valid Audit
				// Since we are walking backwards from the Terminal Leaf,
				// the first valid decision we find is the "Global Winner."
				if !foundOpinion {
					verdict = jsons[i].Accepted()
					foundOpinion = true
				}
			}
		}

		// Move up the chain
		next, ok := allBlocks[current.PrevSignature]
		if !ok {
			break // Reached the Genesis Anchor
		}
		current = next
	}

	if !foundOpinion {
		return false, ErrNoConsensus
	}

	return verdict, nil
}

func parseForJsonBlocks(data string, passthroughMatches ...bool) []Audit {
	var allAudits []Audit
	matches := AuditRegex.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		rawContent := strings.TrimSpace(match[0])

		// Try raw JSON first - this is the BTU 'Truthful' approach
		if json.Valid([]byte(rawContent)) {
			a := Audit{
				Raw: rawContent,
			}
			err := json.Unmarshal([]byte(rawContent), &a)
			if err != nil {
				log.Printf("could not parse audit %s: %s", rawContent, err)
				continue
			}
			if len(passthroughMatches) == 0 || !passthroughMatches[0] {
				err = a.Validate()
				if err != nil {
					log.Printf("could not validate audit %s: %s", rawContent, err)
					continue
				}
			}

			allAudits = append(allAudits, a)
			continue
		}
	}
	return allAudits
}
