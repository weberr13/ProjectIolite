package gemini

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"google.golang.org/genai"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/weberr13/ProjectIolite/brain"
)

var (
	ErrNoKeyFound = errors.New("provide a gemini API key on GEMINI_API_KEY")
	MidTemp       = float32(0.9)
	HighTemp      = float32(1.2)
	NarrowTopP    = float32(.8)
	WideTopP      = float32(0.95)
)

type ContentGenerator interface {
	GenerateContent(context.Context, string, []*genai.Content, *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

type Gemini struct {
	key       string
	cl        *genai.Client
	model     string
	cfg       *genai.ClientConfig
	generator ContentGenerator
	history   brain.ChatHistory
}

func (g *Gemini) Model() string {
	return "gemini" + g.model
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

// Add this internal variable to the package
var (
	newClient   = genai.NewClient
	newClientMX sync.Mutex
)

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

	newClientMX.Lock()
	client, err := newClient(ctx, g.cfg)
	newClientMX.Unlock()
	if err != nil {
		return nil, err
	}
	g.cl = client
	g.generator = g.cl.Models
	return g, nil
}

func (m *Gemini) Dream(ctx context.Context, spec *openapi3.T, sv brain.SignVerifier) ([]brain.SignedHydration, error) {
	p, err := brain.GenerateDreamPrompt(ctx, spec, sv)
	if err != nil {
		return nil, err
	}
	// log.Printf("hydration prompt: %#v\n", p)
	resp, err := m.Think(ctx, sv, brain.Request{
		T: p,
	})
	if err != nil {
		return nil, err
	}
	// log.Printf("hydration result: %#v\n", resp)
	return brain.ParseDreamResponse(ctx, spec, sv, time.Now(), resp.Text().Data)
}

func (m *Gemini) Wake(ctx context.Context, sv brain.SignVerifier, h brain.SignedHydration) error {
	err := h.Verify(sv)
	if err != nil {
		return err
	}
	p, err := brain.GenerateHyrationPrompt(ctx, sv, h)
	if err != nil {
		return err
	}
	resp, err := m.Think(ctx, sv, brain.Request{T: p})
	if err != nil {
		return err
	}
	return brain.ParseHydrationReponse(ctx, sv, resp.Text().Data)
}

func (m *Gemini) Chime(ctx context.Context, sv brain.SignVerifier, c brain.Chimetric) error {
	p, err := brain.GenerateChimePrompt(ctx, sv, c)
	if err != nil {
		return err
	}
	resp, err := m.Think(ctx, sv, brain.Request{T: p})
	if err != nil {
		return err
	}
	return brain.ParseChimeReponse(ctx, sv, resp.Text().Data)
}

func (g *Gemini) genConfig() *genai.GenerateContentConfig {
	return &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: true,
		},
	}
}

func (g *Gemini) getHistory(sv brain.SignVerifier, start int) ([]*genai.Content, error) {
	start = min(len(g.history), start)
	log.Printf("prepending history of %d\n", len(g.history)-start)

	content := []*genai.Content{}
	for i := start; i < len(g.history); i++ {
		if err := g.history[i].Verify(sv); err != nil {
			return nil, err
		}
		content = append(content, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: g.history[i].User.Data}},
		})
		content = append(content, &genai.Content{
			Role:  genai.RoleModel,
			Parts: []*genai.Part{{Text: g.history[i].Model.Data}},
		})
	}
	return content, nil
}

// Think generates the initial response
// Think is stateful (uses chat) where Evaluate is *not*
func (g *Gemini) Think(ctx context.Context, sv brain.SignVerifier, input brain.Request) (brain.Response, error) {
	now := time.Now()
	defer func() {
		log.Printf("Think took: %v", time.Since(now))
	}()
	if g.cl == nil || g.generator == nil {
		return &brain.ErrorResponse{E: errors.New("gemini client not initialized")}, errors.New("gemini client not initialized")
	}
	cfg := g.genConfig()
	cfg.Temperature = &MidTemp
	cfg.TopP = &WideTopP
	inst := genai.Text(brain.ThoughtInstructions)
	if len(inst) == 1 {
		cfg.SystemInstruction = inst[0]
	} else {
		log.Printf("could not generate single part system instruction, instead we got %#v", inst)
	}
	prompt := input.T
	prompt.Namespace = brain.TypePrompt
	err := prompt.Sign(sv)
	if err != nil {
		return &GeminiError{e: err}, err
	}
	content, err := g.getHistory(sv, 0)
	if err != nil {
		return &GeminiError{e: err}, err
	}
	content = append(content, &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: input.Text().Data}},
	})
	result, err := g.generator.GenerateContent(ctx, g.model, content, cfg)
	if err != nil {
		return &GeminiError{e: err}, err
	}
	b, err := brain.NewBaseResponse(sv, "gemini", prompt, input.G...)
	if err != nil {
		return &GeminiError{e: err}, err
	}
	resp := &GeminiResponse{resp: result, model: g.model, BaseResponse: b}
	err = resp.Sign(sv)
	if err == nil {
		log.Printf("Response: %s\n", resp.Text().Data)
		if !input.Stateless {
			g.history.AppendInPlace(resp.Prompt(true), resp.Thought(true))
		}
	}
	return resp, err
}

// Evaluate audits another brain's output
func (g *Gemini) Evaluate(ctx context.Context, sv brain.SignVerifier, peerOutput brain.Response, stateless bool) (brain.Decision, error) {
	now := time.Now()
	defer func() {
		log.Printf("Evaluate took: %v", time.Since(now))
	}()
	if g.cl == nil || g.generator == nil {
		return &brain.ErrorDecision{E: errors.New("gemini client not initialized")}, errors.New("gemini client not initialized")
	}
	cfg := g.genConfig()
	cfg.Temperature = &HighTemp
	cfg.ThinkingConfig.ThinkingLevel = genai.ThinkingLevelHigh
	cfg.TopP = &NarrowTopP
	// if strings.Contains(g.model, "gemini-pro") {
	// 	cfg.CandidateCount = 3
	// } else {
	cfg.CandidateCount = 1
	instruction := genai.Text(brain.GenerateEvalPromptText(sv, false))
	if len(instruction) == 1 {
		cfg.SystemInstruction = instruction[0]
		cfg.Tools = append(cfg.Tools, &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}})
	} else {
		log.Printf("could not generate single part system instruction, instead we got %#v", instruction)
	}
	content, err := g.getHistory(sv, 0)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	content = append(content, &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: peerOutput.Describe(sv)}},
	})
	result, err := g.generator.GenerateContent(ctx, "gemini-pro-latest", content, cfg)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	prev, err := NewDecision(peerOutput, sv)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}

	allThoughts := []brain.Signed{}
	turnThoughts, err := candidatesToThoughts(sv, result, peerOutput.Prompt())
	if err != nil {
		prev.SetError(err)
		return prev, err
	}
	allThoughts = append(allThoughts, turnThoughts...)
	var textBlock brain.Signed
	if len(allThoughts) > 0 {
		textBlock = allThoughts[len(allThoughts)-1].NextUnsigned(result.Text(), brain.TypeText)
	} else {
		textBlock = brain.NewUnsigned(result.Text(), brain.TypeText)
		textBlock.PrevSignature = peerOutput.Prompt().Signature
	}
	err = textBlock.Sign(sv)
	if err != nil {
		prev.SetError(err)
		return prev, err
	}
	err = prev.Add("gemini", allThoughts, textBlock, sv)
	if err != nil {
		return prev, err
	}
	err = prev.Sign(sv)
	if err == nil && !stateless {
		gp := prev.GenesisPrompt()
		p := gp.NextUnsigned(peerOutput.Describe(sv))
		err = p.Sign(sv)
		if err != nil {
			prev.SetError(err)
		} else {
			g.history.AppendInPlace(p, textBlock)
		}
	}
	return prev, err
}
