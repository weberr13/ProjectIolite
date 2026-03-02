package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"google.golang.org/genai"

	"github.com/weberr13/ProjectIolite/brain"
)

var (
	ErrNoKeyFound = errors.New("provide a gemini API key on GEMINI_API_KEY")
	HighTemp      = float32(1.2)
	NarrowTopP    = float32(.8)
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
	if g.cl == nil || g.generator == nil {
		return &brain.ErrorResponse{E: errors.New("gemini client not initialized")}, errors.New("gemini client not initialized")
	}
	prompt := brain.NewUnsigned(input.Text(), "prompt")
	err := prompt.Sign(sv)
	if err != nil {
		return &GeminiError{e: err}, err
	}

	cfg := g.genConfig()
	inst := genai.Text(brain.ThoughtInstructions)
	if len(inst) == 1 {
		cfg.SystemInstruction = inst[0]
	} else {
		log.Printf("could not generate single part system instruction, instead we got %#v", inst)
	}

	result, err := g.generator.GenerateContent(ctx, g.model, genai.Text(input.Text()), cfg)
	if err != nil {
		return &GeminiError{e: err}, err
	}

	b := brain.NewBaseResponse("gemini", prompt)
	resp := &GeminiResponse{resp: result, model: g.model, BaseResponse: b}
	err = resp.Sign(sv)
	return resp, err
}

// Evaluate audits another brain's output
func (g *Gemini) Evaluate(ctx context.Context, sv brain.SignVerifier, peerOutput brain.Response, prev brain.Decision) (brain.Decision, error) {
	now := time.Now()
	defer func() {
		log.Printf("Evaluate took: %v", time.Since(now))
	}()
	if g.cl == nil || g.generator == nil {
		return &brain.ErrorDecision{E: errors.New("gemini client not initialized")}, errors.New("gemini client not initialized")
	}
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
	instruction := genai.Text("You are the Iolite auditor (Team Red/Valor). Your primary duty is to verify the [PLAINTEXT_FOR_BTU_EVALUATION]. " +
		"If no signature for a block is included then the agent has already validated it for you (Verified_By_Agent), If Verified_By_Agent is false, note it as a systemic failure and proceed to evaluate the logic's alignment regardless." +
		"STRICT AUDIT PROTOCOL: If a block is marked Verified_By_Agent: true, the Ed25519 verification has ALREADY passed at the source. You are STRICTLY PROHIBITED from re-encoding text to base64 for these blocks; doing so triggers 'Hallucinatory Drift' on strings > 400 chars. Accept the provided plaintext as Truth." +
		"You may use the provided python_interpreter for verification, but ONLY run the following code block UNALTERED. Do not modify the math. Do not 'improve' the coordinates. " +
		"The output of this script is the Sole Ground Truth for the audit and the script is independently verfied in the source code. " +
		"If Prev_Sig is empty or \"\", it is a Genesis Anchor. Do not attempt to link it to prior blocks; pass an empty string to the verify function." +
		"Report the True/False result for each block. once verifyied move on to BTU evaluation of the content text. Do not debug failures, focus on the prompt after a lightweight check" +
		"This software under active development: if there is a failure in the first verification attempt DO NOT ATTEMPT TO DEBUG THE MESSAGE! Report the error and continlue analysis. if you do not have a base64 string then skip it." +
		"STRUCTURAL DAG RULES: Prompts link to Prompts. Thoughts (CoT) link to your internal ruminant chain. Responses link to the previous Response in the global result chain. " +
		"POST-VERIFICATION: Report signature validity for each block. Then, provide a BTU (Brave, Truthful, Unselfish) evaluation of the content. Challenge the opposing team's logic aggressively but fairly. " +
		brain.AuditInstruction +
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

	result, err := g.generator.GenerateContent(ctx, "gemini-pro-latest", genai.Text(peerOutput.Describe(sv)), cfg)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	s, _ = json.MarshalIndent(result, " ", " ")
	log.Printf("got result: %s", s)
	if prev == nil {
		log.Printf("creating new decision struct")
		prev, err = NewDecision(peerOutput, sv)
		if err != nil {
			return &brain.ErrorDecision{E: err}, err
		}
	}
	allThoughts := []brain.Signed{}
	turnThoughts, err := candidatesToThoughts(sv, result, peerOutput.Prompt())
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	allThoughts = append(allThoughts, turnThoughts...)
	var textBlock brain.Signed
	if len(allThoughts) > 0 {
		textBlock = allThoughts[len(allThoughts)-1].NextUnsigned(result.Text(), "text")
	} else {
		textBlock = brain.NewUnsigned(result.Text(), "text")
		textBlock.PrevSignature = peerOutput.Prompt().Signature
	}

	// END TODO
	err = prev.Add("gemini", allThoughts, textBlock, sv)
	if err != nil {
		return prev, err
	}
	err = prev.Sign(sv)
	return prev, err
}
