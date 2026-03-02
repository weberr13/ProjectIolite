package brain

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

type DecisionParser struct{}

type HasTexts interface {
	Cots() map[string][][]Signed  // map of model -> collections of CoTs indexed by rebuttal turn order
	Prompts() map[string][]Signed // the sequence of prompts given to each model
	Texts() map[string][]Signed   // map of model -> turn responses
}

func (p *DecisionParser) GetAudits(sv SignVerifier, dec HasTexts) (Audits, error) {
	allBlocks := make(map[string]Signed)
	hasParent := make(map[string]bool)
	textLeaves := make(map[string]Signed)

	// 🛡️ [PROVENANCE TRACKER]: Map Signature -> Model Name
	identities := make(map[string]string)

	// 1. Flatten the Universe & Identify text-specific candidates
	ingest := func(blocks []Signed, modelName string, isTextNamespace bool) {
		for _, b := range blocks {
			allBlocks[b.Signature] = b
			identities[b.Signature] = modelName // 👈 Capture the 'Physical' Author
			if b.PrevSignature != "" {
				hasParent[b.PrevSignature] = true
			}
			if isTextNamespace {
				textLeaves[b.Signature] = b
			}
		}
	}

	for name, p := range dec.Prompts() {
		ingest(p, name, false)
	}
	for name, c := range dec.Cots() {
		for _, turn := range c {
			ingest(turn, name, false)
		}
	}
	for name, t := range dec.Texts() {
		ingest(t, name, true)
	}

	// 2. Resolve the 'Sovereign Terminal'
	// We only want a leaf if it belongs to the Text namespace AND has no children.
	found := false
	var terminal Signed
	maxDepth := -1

	for sig, block := range textLeaves {
		if !hasParent[sig] {
			// Calculate depth by walking up to the root
			depth := 0
			curr := block
			for curr.PrevSignature != "" {
				parent, ok := allBlocks[curr.PrevSignature]
				if !ok {
					break
				}
				curr = parent
				depth++
			}

			// ⚖️ [DETERMINISM]: Deepest node in the text namespace wins
			// If depths are equal, use Lexicographical Sort on Signature as a tie-breaker
			if depth > maxDepth || (depth == maxDepth && block.Signature > terminal.Signature) {
				terminal = block
				maxDepth = depth
				found = true
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("braid rupture: no terminal leaf found in 'text' namespace")
	}

	// 3. Trace the Lineage Backwards
	var causalBraid []Audit
	current := terminal
	for {
		if err := current.Verify(sv); err != nil {
			return nil, fmt.Errorf("integrity violation at %s: %w", current.Signature, err)
		}

		payload := current.Data
		if decoded, err := base64.StdEncoding.DecodeString(current.Data); err == nil {
			payload = string(decoded)
		}

		// Extract Audits from the verified block
		blockAudits := parseForJsonBlocks(payload)
		for i := len(blockAudits) - 1; i >= 0; i-- {
			auditorName := identities[current.Signature]
			blockAudits[i].Auditor = auditorName
			// ⚖️ [TARGET SEARCH]: Find the target of this audit
			// Walk back from 'current' to find the first Text block
			// NOT authored by 'auditorName'
			targetSearch := current
			for {
				if targetSearch.PrevSignature == "" {
					break // Hit Genesis
				}
				parent, ok := allBlocks[targetSearch.PrevSignature]
				if !ok {
					break // Rupture
				}

				parentName := identities[parent.Signature]
				// 🛡️ [THE CRITICAL CHECK]:
				// Is this a Text block? AND is it a different author?
				_, isText := textLeaves[parent.Signature]
				if isText && parentName != auditorName {
					blockAudits[i].Author = parentName
					break
				}
				targetSearch = parent
			}

			causalBraid = append(causalBraid, blockAudits[i])
		}

		if current.PrevSignature == "" {
			break
		}
		parent, ok := allBlocks[current.PrevSignature]
		if !ok {
			return nil, fmt.Errorf("dangling link: %s -> %s", current.Signature, current.PrevSignature)
		}
		current = parent
	}

	if len(causalBraid) == 0 {
		return Audits{}, nil
	}
	// 4. Final Chronological Ordering
	for i, j := 0, len(causalBraid)-1; i < j; i, j = i+1, j-1 {
		causalBraid[i], causalBraid[j] = causalBraid[j], causalBraid[i]
	}
	return causalBraid, nil
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
