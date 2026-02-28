package brain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
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
	if sv == nil {
		return false, ErrNoConsensus
	}
	texts := dec.Texts()

	// TODO: once the prompt signs the text chain find the prompt with PrevSignature == "" and use it
	PromptSignature := ""

	// 1. Flatten all blocks into a map for O(1) lookups by PrevSignature
	chainMap := make(map[string]Signed)
	var genesis Signed
	for _, txs := range texts {
		for _, tx := range txs {
			if tx.PrevSignature == PromptSignature {
				genesis = tx
			}
			chainMap[tx.PrevSignature] = tx
		}
	}

	// 2. Walk the braid from Genesis
	current := genesis
	found := false
	lastDecision := false

	for {
		// Verify integrity at every step of the walk
		if err := current.Verify(sv); err != nil {
			return false, err
		}

		// 1. ALL-OR-NOTHING DECODING:
		// We attempt to decode the entire Data field.
		// If it's not B64, we treat it as raw (for transition/legacy).
		payload := current.Data
		if decoded, err := base64.StdEncoding.DecodeString(current.Data); err == nil {
			payload = string(decoded)
		}

		// Parse JSON from the current block
		jsons := parseForJsonBlocks(payload)
		for _, b := range jsons {
			var m map[string]any
			if err := json.Unmarshal(b, &m); err == nil {
				val, ok := m["accepted"].(bool)
				if !ok {
					val, ok = m["approved"].(bool)
				}
				if ok {
					found = true
					lastDecision = val // Overwrite: The "Last Authoritative" rule
				}
			}
		}

		// Look for the next link in the chain
		next, ok := chainMap[current.Signature]
		if !ok {
			break // End of the braid
		}
		current = next
	}

	if !found {
		return false, errors.New("no terminal decision found in braid")
	}

	return lastDecision, nil
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
