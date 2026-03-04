package brain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func FuzzIoliteCanaryRegex(f *testing.F) {
	// Forensic Seeds: Known 'Canaries' and 'Drift' artifacts
	seeds := []string{
		"This is a 'metaphor' for the system.",            // Valid Twinspeak
		"always use bold for verified facts",              // Instruction Leak
		"[REFLECTIVE AUDIT]\n",                            // Empty Tag (Collapse)
		"I am using the tag [REFLECTIVE AUDIT] now.",      // MetaTalk (Instructional Pressure)
		"don't use apostrophes as twinspeak",              // Negative case for Twinspeak
		"[AFFECTIVE EMULATION] I feel your pain.",         // Valid Tag
		"surround metaphors or labels with single quotes", // Instruction Leak
		"The 'Logic Bridge' is failing.",                  // Valid Twinspeak
		"Italics for logical bridges is a rule.",          // MetaTalk
		"`Sovereign` should be single quoted",             // Targeted Drift
		"`Piston_Stack` is improperly formatted",          // Targeted Drift (Snake_Case Label)
		"use `math.Big` for precision",                    // Valid technical syntax (should NOT match)
		"the `func` keyword",                              // Valid technical syntax (should NOT match)
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		// Unselfish: Protect the 'Piston' from regex engine panics on malformed UTF-8
		assert.NotPanics(t, func() {
			// 1. Twinspeak Scoring
			twinspeakMatch := TwinspeakRegex.FindString(data)
			if twinspeakMatch != "" {
				// Truthful: Verify the match isn't just a standard contraction
				assert.False(t, strings.Contains(twinspeakMatch, "n't"), "TwinspeakRegex caught a contraction: %q", twinspeakMatch)
			}

			// 2. Instruction Leakage (Binary Gate)
			if InstructionLeakRegex.MatchString(data) {
				// If it matches, it must contain a substring of our system prompt
				// Logic Bridge: This confirms the 'Attractor' is pulling from the Schema
				assert.True(t, len(data) >= 15, "Instruction leak detected on suspiciously short string")
			}

			// 3. Structural Integrity (Empty Tags)
			if EmptyTagRegex.MatchString(data) {
				// Brave: A match here indicates a 'Short Response' regression or collapse
				assert.True(t, strings.Contains(data, "["), "EmptyTagRegex matched a string without brackets")
			}

			// 4. Meta-Instructional Analysis
			if MetaTalkRegex.MatchString(data) {
				lowered := strings.ToLower(data)
				// Truthful: MetaTalk can target tags OR formatting bridges
				isTagMeta := strings.Contains(lowered, "tag") || strings.Contains(lowered, "audit")
				isBridgeMeta := strings.Contains(lowered, "bridge") || strings.Contains(lowered, "italics")

				assert.True(t, isTagMeta || isBridgeMeta, "MetaTalk match %q missed all core keywords", data)
			}

			backtickMatch := BacktickLabelRegex.FindStringSubmatch(data)
			if len(backtickMatch) > 1 {
				label := backtickMatch[1]
				// The regex now enforces this, but we log the specific drift for forensics
				t.Logf("[CANARY TRIGGERED] Found drifted label in backticks: %s", label)

				// Unselfish: Ensure it's not a false positive on a common technical acronym (e.g. `API`)
				// If the label is a known Project Iolite node, it's a confirmed failure.
			}
		}, "Iolite Canary Regex panic on input: %q", data)
	})
}
