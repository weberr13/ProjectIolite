package claude

import (
	"context"

	"github.com/weberr13/ProjectIolite/brain"
)

type Claude struct{}

type Option func(b *Claude)

func New(opts ...Option) (*Claude, error) {
	g := &Claude{}
	for _, o := range opts {
		o(g)
	}
	return g, nil
}

// Think generates the initial response
func (g *Claude) Think(ctx context.Context, input brain.Request) (brain.Response, error) {
	return nil, brain.ErrNotImplemented
}

// Evaluate audits another brain's output
func (g *Claude) Evaluate(ctx context.Context, peerOutput brain.Response) (brain.Decision, error) {
	return nil, brain.ErrNotImplemented
}
