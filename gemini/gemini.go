package gemini

import (
	"context"
	"errors"

	"github.com/weberr13/ProjectIolite/brain"
)

var ErrNoKeyFound = errors.New("provide a gemini API key on GEMINI_API_KEY")

type Gemini struct {
	key string
}

type Option func(b *Gemini)

func New(apikey string, opts ...Option) (*Gemini, error) {
	if apikey == "" {
		return nil, ErrNoKeyFound
	}
	g := &Gemini{
		key: apikey,
	}
	for _, o := range opts {
		o(g)
	}
	return g, nil
}

// Think generates the initial response
func (g *Gemini) Think(ctx context.Context, input brain.Request) (brain.Response, error) {
	return nil, brain.ErrNotImplemented
}

// Evaluate audits another brain's output
func (g *Gemini) Evaluate(ctx context.Context, peerOutput brain.Response) (brain.Decision, error) {
	return nil, brain.ErrNotImplemented
}
