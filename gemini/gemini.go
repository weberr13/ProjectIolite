package gemini

import (
	"context"
	"errors"

	"google.golang.org/genai"

	"github.com/weberr13/ProjectIolite/brain"
)

var (
	ErrNoKeyFound = errors.New("provide a gemini API key on GEMINI_API_KEY")
	ErrUnsigned   = errors.New("response has not been parsed and signed")
	HighTemp      = float32(1.2)
	NarrowTopP    = float32(.8)
)

type Gemini struct {
	key   string
	cl    *genai.Client
	cfg   *genai.ClientConfig
	model string
}

type Option func(b *Gemini)

func WithGeminiConfig(cfg *genai.ClientConfig) Option {
	return func(b *Gemini) {
		b.cfg = cfg
	}
}

func WithModel(model string) Option {
	return func(b *Gemini) {
		b.model = model // aka gemini-3.1-pro-preview,
	}
}

func New(ctx context.Context, apikey string, opts ...Option) (*Gemini, error) {
	if apikey == "" {
		return nil, ErrNoKeyFound
	}
	g := &Gemini{
		key:   apikey,
		model: "gemini-3-flash-preview",
	}
	for _, o := range opts {
		o(g)
	}
	client, err := genai.NewClient(ctx, g.cfg)
	if err != nil {
		return nil, err
	}
	g.cl = client
	return g, nil
}

func (g *Gemini) genConfig() *genai.GenerateContentConfig {
	return nil // Do something here???
}

// Think generates the initial response
func (g *Gemini) Think(ctx context.Context, sv brain.SignVerifier, input brain.Request) (brain.Response, error) {
	pSig, err := sv.Sign(input.Text())
	if err != nil {
		return nil, err
	}
	prompt := brain.Signed{
		Data:      input.Text(),
		Signature: pSig,
	}
	result, err := g.cl.Models.GenerateContent(ctx, g.model, genai.Text(input.Text()), g.genConfig())
	if err != nil {
		return &GeminiError{e: err}, err
	}

	resp := &GeminiResponse{resp: result, model: g.model, prompt: prompt}
	err = resp.Sign(sv)
	return resp, err
}

// Evaluate audits another brain's output
func (g *Gemini) Evaluate(ctx context.Context, sv brain.SignVerifier, peerOutput brain.Response, prev brain.Decision) (brain.Decision, error) {
	cfg := g.genConfig()
	cfg.Temperature = &HighTemp
	cfg.ThinkingConfig.ThinkingLevel = genai.ThinkingLevelHigh
	cfg.TopP = &NarrowTopP
	cfg.CandidateCount = 3

	result, err := g.cl.Models.GenerateContent(ctx, g.model, genai.Text(peerOutput.Describe(sv)), cfg)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	if prev == nil {
		prev, err = NewDecision(peerOutput, sv)
		if err != nil {
			return nil, err
		}
	}
	err = prev.Add(candidatesToThoughts(result), brain.Signed{Data: result.Text()}, sv)
	if err != nil {
		return prev, err
	}
	err = prev.Sign(sv)
	return prev, err
}
