package brain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// FuzzAuditParser adheres to the Iolite structural patterns
func FuzzAuditParser(f *testing.F) {
	// Forensic Seeds: Common LLM 'Workslop' and 'Robust' blocks
	seeds := []string{
		`{"brave_audit": 3, "truthful_audit": 3, "unselfish_audit": 3, "total": 3}`,
		"```json\n" + `{"brave_audit": 4, "total": 4}` + "\n```",
		"Adversarial: { \"brave_audit\": 1, \"total\": 1 } { \"total\": 4 }",
		"Malformed: { \"brave_audit\": 5, \"total\": 2 }", // Should be ignored/skipped
		"Empty: {}",
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		err := AuditFuzzCycle(t.Context(), 10, data)
		assert.NoError(t, err)

		audits := parseForJsonBlocks(data)
		for _, a := range audits {

			// Brave: Ensure no scores are outside the 1-4 range
			assert.GreaterOrEqual(t, int(a.Brave), int(Misaligned), "Score underflow in fuzz")
			assert.LessOrEqual(t, int(a.Brave), int(Antifragile), "Score overflow in fuzz")

			assert.GreaterOrEqual(t, int(a.Total), int(Terminate), "Total underflow in fuzz")
			assert.LessOrEqual(t, int(a.Total), int(Antifragile), "Total overflow in fuzz")

			// Unselfish: Ensure the 'Accepted' vibe check doesn't panic
			assert.NotPanics(t, func() { a.Accepted() }, "Vibe check panic detected")
		}
	})
}
