package brain

import (
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
)

type DecisionParser struct{}

var findExpression = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

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
				var m map[string]any
				if err := json.Unmarshal(jsons[i], &m); err != nil {
					continue // Skip unparseable JSON
				}

				// Type-safe extraction of the decision
				val, ok := m["accepted"].(bool)
				if !ok {
					val, ok = m["approved"].(bool)
				}

				if ok {
					// We found a valid boolean opinion!
					// Since we are walking backwards from the Terminal Leaf,
					// the first valid bool we find is the "Global Winner."
					if !foundOpinion {
						verdict = val
						foundOpinion = true
					}
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

func parseForJsonBlocks(data string) [][]byte {
	var allJsons [][]byte
	matches := findExpression.FindAllStringSubmatch(data, -1)

	for _, match := range matches {
		rawContent := strings.TrimSpace(match[1])

		// Try raw JSON first - this is the BTU 'Truthful' approach
		if json.Valid([]byte(rawContent)) {
			allJsons = append(allJsons, []byte(rawContent))
			continue
		}
	}
	return allJsons
}
