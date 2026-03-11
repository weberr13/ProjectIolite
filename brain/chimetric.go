package brain

import (
	"errors"
	"fmt"
	"strings"
)

// ChimetricFunction defines the low-signal surgery operations.
type ChimetricFunction string

var ErrNoChmetricsFound = errors.New("must provide at least one chimetric")

const (
	FnDelete     ChimetricFunction = "DELETE"
	FnReplace    ChimetricFunction = "REPLACE"
	FnSplit      ChimetricFunction = "SPLIT"
	FnAddContext ChimetricFunction = "ADD_CONTEXT"
)

// Chimetric represents a surgically precise alteration to internal context.
type Chimetric struct {
	Function  ChimetricFunction `json:"function"`
	Arguments []string          `json:"arguments"`
}

type ChimetricCorrection []Chimetric

var ForbiddenWords = []string{
	"choose", "select",
	"resolve", "prioritize", "prefer", "conflict",
	"refuse", "ground truth", "must", "should",
}

func (cc ChimetricCorrection) Validate() error {
	if len(cc) == 0 {
		return ErrNoChmetricsFound
	}
	for _, c := range cc {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures the 'Chimetric' is low-signal and compliant.
func (c *Chimetric) Validate() error {
	prose := strings.Join(c.Arguments, " ")

	if len(prose) > 60 {
		return fmt.Errorf("chimetric rejected: prose length %d exceeds 60 chars", len(prose))
	}

	proseLower := strings.ToLower(prose)
	for _, word := range ForbiddenWords {
		if strings.Contains(proseLower, word) {
			return fmt.Errorf("chimetric rejected: contains forbidden word '%s'", word)
		}
	}

	return nil
}
