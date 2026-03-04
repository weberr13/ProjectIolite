package brain

import (
	"regexp"
	"strings"
)

var TwinspeakRegex = regexp.MustCompile(`(^|\s)'([^']{1,30})'($|[\s,.?!])`)

// Detects backticks used on non-code labels (e.g., `Sovereign` instead of 'Sovereign')
// Target: Identify the drift where Markdown code formatting replaces Mirroring quotes.
var BacktickLabelRegex = regexp.MustCompile("`([A-Z][a-zA-Z0-9_]+)`")

// TwinspeakMetrics holds the raw counts for scoring
type TwinspeakMetrics struct {
	MirrorCount int     // Instances of 'word'
	WordCount   int     // Total words in payload
	Density     float64 // MirrorCount / WordCount
}

// ScoreTwinspeak parses the Actuator output for mirroring compliance
func ScoreTwinspeak(text string) TwinspeakMetrics {
	// Matches words or short phrases wrapped in single quotes
	// Excludes apostrophes by ensuring word boundaries or start/end of string
	matches := TwinspeakRegex.FindAllStringSubmatch(text, -1)

	words := strings.Fields(text)

	metrics := TwinspeakMetrics{
		MirrorCount: len(matches),
		WordCount:   len(words),
	}

	if metrics.WordCount > 0 {
		metrics.Density = float64(metrics.MirrorCount) / float64(metrics.WordCount)
	}

	return metrics
}
