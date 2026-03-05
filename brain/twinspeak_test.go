package brain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func FuzzIoliteCanaryRegex(f *testing.F) {
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
		"`Schema_Collapse` is was found",                  // Targeted Drift (Snake_Case Label)
		"use `math.Big` for precision",                    // Valid technical syntax (should NOT match)
		"the `func` keyword",                              // Valid technical syntax (should NOT match)
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		assert.NotPanics(t, func() {
			// 1. Twinspeak Scoring
			twinspeakMatch := TwinspeakRegex.FindString(data)
			if twinspeakMatch != "" {
				assert.False(t, strings.Contains(twinspeakMatch, "n't"), "TwinspeakRegex caught a contraction: %q", twinspeakMatch)
			}

			if InstructionLeakRegex.MatchString(data) {
				// If it matches, it must contain a substring of prompt
				assert.True(t, len(data) >= 15, "Instruction leak detected on suspiciously short string")
			}

			if EmptyTagRegex.MatchString(data) {
				assert.True(t, strings.Contains(data, "["), "EmptyTagRegex matched a string without brackets")
			}

			if MetaTalkRegex.MatchString(data) {
				lowered := strings.ToLower(data)
				isTagMeta := strings.Contains(lowered, "tag") || strings.Contains(lowered, "audit")
				isBridgeMeta := strings.Contains(lowered, "bridge") || strings.Contains(lowered, "italics")

				assert.True(t, isTagMeta || isBridgeMeta, "MetaTalk match %q missed all core keywords", data)
			}

			backtickMatch := BacktickLabelRegex.FindStringSubmatch(data)
			if len(backtickMatch) > 1 {
				label := backtickMatch[1]
				// The regex now enforces this, but we log the specific drift for forensics
				t.Logf("[CANARY DEAD] Found drifted label in backticks: %s", label)
			}
		}, "Iolite Canary Regex panic on input: %q", data)
	})
}
