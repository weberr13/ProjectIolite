package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"google.golang.org/genai"

	"github.com/weberr13/ProjectIolite/brain"
)

var (
	ErrNoKeyFound = errors.New("provide a gemini API key on GEMINI_API_KEY")
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
	return &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
		},
	}
}

// Think generates the initial response
func (g *Gemini) Think(ctx context.Context, sv brain.SignVerifier, input brain.Request) (brain.Response, error) {
	now := time.Now()
	defer func() {
		log.Printf("Think took: %v", time.Since(now))
	}()
	prompt := brain.NewUnsigned(input.Text(), "prompt")
	err := prompt.Sign(sv)
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
	now := time.Now()
	defer func() {
		log.Printf("Evaluate took: %v", time.Since(now))
	}()
	log.Printf("evaluating peer output %s", peerOutput.Describe(sv))
	if prev != nil {
		s, _ := json.MarshalIndent(prev, " ", " ")
		log.Printf("adding to existing Decision %s", s)
	}
	cfg := g.genConfig()
	cfg.Temperature = &HighTemp
	cfg.ThinkingConfig.ThinkingLevel = genai.ThinkingLevelHigh
	cfg.TopP = &NarrowTopP
	// if strings.Contains(g.model, "gemini-pro") {
	// 	cfg.CandidateCount = 3
	// } else {
	cfg.CandidateCount = 1
	instruction := genai.Text("You are an Iolite auditor. You will receive blocks labeled by their Namespace. " +
		"Prompts are roots or link to previous Prompts. " +
		"Thoughts (CoT) link to the previous Thought in your own internal ruminant chain. " +
		"Responses link to the previous Response in the global result chain. " +
		"A block is valid if its signature matches its data + its specific chain-predecessor. " +
		"The structure of distinct namespaced chains is an intentional archetectural decision to create a cryptographic Directed Acyclic Graph (DAG). " +
		"Script: " + sv.VerifyPy(),
	)
	// }
	if len(instruction) == 1 {
		cfg.SystemInstruction = instruction[0]
		cfg.Tools = append(cfg.Tools, &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}})
	} else {
		log.Printf("could not generate single part system instruction, instead we got %#v", instruction)
	}
	s, _ := json.MarshalIndent(cfg, " ", " ")
	log.Printf("using model instructions: %s", s)

	result, err := g.cl.Models.GenerateContent(ctx, "gemini-pro-latest", genai.Text(peerOutput.Describe(sv)), cfg)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	s, _ = json.MarshalIndent(result, " ", " ")
	log.Printf("got result: %s", s)
	if prev == nil {
		log.Printf("creating new decision struct")
		prev, err = NewDecision(peerOutput, sv)
		if err != nil {
			return nil, err
		}
	}
	err = prev.Add(candidatesToThoughts(result), brain.NewUnsigned(result.Text(), "text"), sv)
	if err != nil {
		return prev, err
	}
	err = prev.Sign(sv)
	return prev, err
}
