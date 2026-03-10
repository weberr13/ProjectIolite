package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/weberr13/ProjectIolite/brain"
)

type MessageGenerator interface {
	New(ctx context.Context, params anthropic.MessageNewParams, opts ...option.RequestOption) (*anthropic.Message, error)
}

type Claude struct {
	cl        *anthropic.Client
	generator MessageGenerator // The Sliver Interface
	runner    ScriptRunner
	history   brain.ChatHistory
}

func (g *Claude) Model() string {
	return "claude"
}

type Option func(b *Claude)

func New(apiKey string, opts ...Option) (*Claude, error) {
	c := &Claude{
		runner: &RealRunner{},
	}
	for _, o := range opts {
		o(c)
	}
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)
	c.cl = &client
	c.generator = &c.cl.Messages
	return c, nil
}

func (m *Claude) Dream(ctx context.Context, spec *openapi3.T, sv brain.SignVerifier) ([]brain.SignedHydration, error) {
	p, err := brain.GenerateDreamPrompt(ctx, spec, sv)
	if err != nil {
		return nil, err
	}
	resp, err := m.Think(ctx, sv, brain.Request{
		T: p,
	})
	return brain.ParseDreamResponse(ctx, spec, sv, time.Now(), resp.Text().Data)
}

func (m *Claude) Wake(ctx context.Context, sv brain.SignVerifier, h brain.SignedHydration) error {
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

func (m *Claude) Chime(ctx context.Context, sv brain.SignVerifier, c brain.Chimetric) error {
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

func (c *Claude) getHistory(sv brain.SignVerifier, start int) ([]anthropic.MessageParam, error) {
	start = min(len(c.history), start)

	log.Printf("prepending history of %d\n", len(c.history)-start)
	content := []anthropic.MessageParam{}
	for i := start; i < len(c.history); i++ {
		if err := c.history[i].Verify(sv); err != nil {
			return nil, err
		}
		content = append(content, anthropic.NewUserMessage(anthropic.NewTextBlock(c.history[i].User.Data)))
		content = append(content, anthropic.NewAssistantMessage(anthropic.NewTextBlock(c.history[i].Model.Data)))
	}
	return content, nil
}

// Think generates the initial response
func (c *Claude) Think(ctx context.Context, sv brain.SignVerifier, input brain.Request) (brain.Response, error) {
	now := time.Now()
	defer func() {
		log.Printf("Think took: %v", time.Since(now))
	}()
	if c.generator == nil {
		return &brain.ErrorResponse{E: errors.New("claude client not initialized")}, errors.New("claude client not initialized")
	}
	chain, err := c.getHistory(sv, 0)
	if err != nil {
		return &ClaudeError{e: err}, err
	}
	prompt := input.T
	prompt.Namespace = brain.TypePrompt
	err = prompt.Sign(sv)
	if err != nil {
		return &ClaudeError{e: err}, err
	}
	chain = append(chain, anthropic.NewUserMessage(anthropic.NewTextBlock(input.Text().Data)))
	message, err := c.generator.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 1024 * 4,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 1024,
				Type:         "enabled",
			},
		},
		System: []anthropic.TextBlockParam{
			{Text: brain.ThoughtInstructions},
		},
		Messages: chain,
		Model:    anthropic.ModelClaudeOpus4_6,
	})
	if err != nil {
		return &ClaudeError{e: err}, err
	}

	b, err := brain.NewBaseResponse(sv, "claude", prompt, input.G...)
	if err != nil {
		return nil, err
	}
	resp := &ClaudeResponse{
		BaseResponse: b,
		resp:         message, model: string(message.Model),
	}
	err = resp.Sign(sv)
	if err == nil && !input.Stateless {
		c.history.AppendInPlace(resp.Prompt(true), resp.Thought(true))
	}
	return resp, err
}

func extractText(msg *anthropic.Message) string {
	var sb strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

func (c *Claude) pytool() *anthropic.ToolParam {
	return &anthropic.ToolParam{
		Name:        "python_interpreter",
		Description: param.Opt[string]{Value: "Execute Python code in a sandboxed environment to verify cryptographic signatures."},
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "The Python code to execute.",
				},
			},
			Required: []string{"code"},
			Type:     "object",
		},
	}
}

type ScriptRunner interface {
	Run(ctx context.Context, code string) (string, error)
}

// RealRunner is the production implementation
type RealRunner struct{}

func (r *RealRunner) Run(ctx context.Context, code string) (string, error) {
	cmd := exec.CommandContext(ctx, "python3", "-")
	cmd.Stdin = strings.NewReader(code)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("PYTHON ERROR:\n%s\n%s", stderr.String(), err.Error())
	}
	return stdout.String(), nil
}

func executePython(ctx context.Context, runner ScriptRunner, tooluse anthropic.ToolUseBlock) (string, error) {
	var input struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(tooluse.Input, &input); err != nil {
		return "", fmt.Errorf("failed to parse tool input: %w", err)
	}

	output, err := runner.Run(ctx, input.Code)
	if err != nil {
		// We return the error string as the result so the LLM can recover
		return err.Error(), nil
	}

	return output, nil
}

var pyImportRegex = regexp.MustCompile(`(?m)^import\s+|^from\s+|^[a-zA-Z_][a-zA-Z0-9_]*\s*=`)

func commentsToCoT(sv brain.SignVerifier, message *anthropic.ToolUseBlock, previous brain.Signed) ([]brain.Signed, error) {
	b := message.JSON.Input.Raw()
	m := map[string]any{}
	err := json.Unmarshal([]byte(b), &m)
	if err != nil {
		log.Printf("could not extract raw script from ToolUseBlock raw: \"%s\", err: \"%s\"", b, err)
		return nil, nil
	}
	code, ok := m["code"].(string)
	if !ok {
		log.Printf("could not extract raw script from ToolUseBlock raw: \"%s\", json: \"%#v\"", b, code)
		return nil, nil
	}
	allThoughts := []brain.Signed{}
	switch message.Name {
	case "python_interpreter":
		// A single thought block is found above the python imports in some scritps
		loc := pyImportRegex.FindStringIndex(code)

		if loc == nil {
			log.Printf("could not parse raw script from ToolUseBlock raw: \"%s\", json: \"%#v\"", b, code)
		}
		thoughtText := strings.TrimSpace(code[:loc[0]])
		if len(thoughtText) > 0 {
			thought := previous.NextUnsigned(thoughtText)
			thought.IsShadow = true
			err := thought.Sign(sv)
			if err != nil {
				return nil, err
			}
			allThoughts = append(allThoughts, thought)
		}
	}
	return allThoughts, nil
}

// Evaluate audits another brain's output
func (c *Claude) Evaluate(ctx context.Context, sv brain.SignVerifier, peerOutput brain.Response, stateless bool) (brain.Decision, error) {
	now := time.Now()
	defer func() {
		log.Printf("Evaluate took: %v", time.Since(now))
	}()
	if c.generator == nil {
		return &brain.ErrorDecision{E: errors.New("claude client not initialized")}, errors.New("claude client not initialized")
	}
	err := peerOutput.Sign(sv)
	instruction := brain.GenerateEvalPromptText(sv, false) // TODO programatic control for long prompts with extra validation
	chain, err := c.getHistory(sv, 0)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	chain = append(chain, anthropic.NewUserMessage(anthropic.NewTextBlock(peerOutput.Describe(sv))))
	params := anthropic.MessageNewParams{
		MaxTokens: 1024 * 16,
		System: []anthropic.TextBlockParam{
			{Text: instruction},
		},
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 1024 * 2,
				Type:         "enabled",
			},
		},
		Tools: []anthropic.ToolUnionParam{
			{OfTool: c.pytool()},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(peerOutput.Describe(sv))),
		},
		Temperature: param.Opt[float64]{Value: 1.0},
		Model:       anthropic.ModelClaudeOpus4_6,
	}

	message, err := c.generator.New(ctx, params)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}

	maxRetries := 10 // should this be limited by the number of the checksums?
	i := 0
	var allThoughts []brain.Signed // <--- The Reservoir
	var allToolRequests []brain.Signed
	var allToolReplies []brain.Signed
	for i = range maxRetries {
		if message == nil {
			break
		}
		// 1. Capture any thinking from THIS turn
		prevThought := peerOutput.Prompt()
		if len(allThoughts) > 0 {
			prevThought = allThoughts[len(allThoughts)-1]
		}
		turnThoughts, err := candidatesToThoughts(sv, message, prevThought)
		if err != nil {
			return &brain.ErrorDecision{E: err}, err
		}
		allThoughts = append(allThoughts, turnThoughts...)

		if message.StopReason != "tool_use" {
			log.Printf("done with tool use on iteration %d", i)
			// s, _ := json.MarshalIndent(message, " ", " ")
			// log.Printf("got result: %s", s)
			break
		}
		prevThought = peerOutput.Prompt()
		if len(allThoughts) > 0 {
			prevThought = allThoughts[len(allThoughts)-1]
		}
		turnThoughts, err = candidatesToThoughts(sv, message, prevThought, true)
		if err != nil {
			return &brain.ErrorDecision{E: err}, err
		}
		allThoughts = append(allThoughts, turnThoughts...)

		historyBlock := anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleAssistant,
			Content: make([]anthropic.ContentBlockParamUnion, len(message.Content)),
		}
		for i, block := range message.Content {
			historyBlock.Content[i] = block.ToParam()
		}
		params.Messages = append(params.Messages, historyBlock)

		var toolResults []anthropic.ContentBlockParamUnion
		for _, block := range message.Content {
			if block.Type == "tool_use" {
				toolUse := block.AsToolUse()
				prevThought = peerOutput.Prompt()
				if len(allThoughts) > 0 {
					prevThought = allThoughts[len(allThoughts)-1]
				}
				commentThoughts, err := commentsToCoT(sv, &toolUse, prevThought)
				if err != nil {
					log.Printf("got error trying to extract script thoughts: %s", err)
				} else if len(commentThoughts) > 0 {
					log.Printf("found thinking in script pre-amble, likely a hidden CoT %v", &commentThoughts)
					allThoughts = append(allThoughts, commentThoughts...)
				}
				prevTool := peerOutput.Prompt()
				if len(allToolRequests) > 0 {
					prevTool = allToolRequests[len(allToolRequests)-1]
				}
				s, _ := json.Marshal(toolUse)
				allToolRequests = append(allToolRequests, prevTool.NextUnsigned(string(s), brain.TypeToolCall))
				// log.Printf("tool request: %#v", toolUse)
				result, err := executePython(ctx, c.runner, toolUse)
				isErr := false
				if err != nil {
					isErr = true
					result = err.Error()
					log.Printf("error executing tool: %s", err)
				}
				prevTool = peerOutput.Prompt()
				if len(allToolReplies) > 0 {
					prevTool = allToolReplies[len(allToolReplies)-1]
				}
				allToolReplies = append(allToolReplies, prevTool.NextUnsigned(string(result), brain.TypeToolResult))
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result, isErr))
			}
		}
		// log.Printf("tool Results being sent back %#v \n", toolResults)
		if len(toolResults) > 0 {
			params.Messages = append(params.Messages, anthropic.NewUserMessage(toolResults...))
		}
		message, err = c.generator.New(ctx, params)
		if err != nil {
			return &brain.ErrorDecision{E: err}, err
		}
	}
	if message == nil {
		return &brain.ErrorDecision{E: errors.New("unknown error, nil message")}, errors.New("unknown error, nil message")
	}
	if i >= maxRetries {
		log.Printf("gave up on %d tool calls", i)
	}
	if message.StopReason == "max_tokens" {
		err := fmt.Errorf("exceeded max tokens by using %d", message.Usage.OutputTokens)
		return &brain.ErrorDecision{E: err}, err
	}

	prevThought := peerOutput.Prompt()
	if len(allThoughts) > 0 {
		prevThought = allThoughts[len(allThoughts)-1]
	}
	turnThoughts, err := candidatesToThoughts(sv, message, prevThought)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	allThoughts = append(allThoughts, turnThoughts...)
	// END TODO

	// log.Printf("creating new decision struct")
	prev, err := NewDecision(peerOutput, sv)
	if err != nil {
		// log.Printf("error creating decision struct")
		return &brain.ErrorDecision{E: err}, err
	}
	var textblock brain.Signed
	if message.StopReason == "refusal" {
		s, _ := json.MarshalIndent(message, " ", " ")
		log.Printf("got refusal result: %s", s)
		err := fmt.Errorf("refusing to answer is an answer: %#v", message)
		prev.SetError(err)
		textblock = brain.NewUnsigned(err.Error(), brain.TypeText)
	} else {
		textblock = brain.NewUnsigned(extractText(message), brain.TypeText)
	}
	// STITCHING: Link Claude's audit to peer's Text Response
	textblock.PrevSignature = peerOutput.Text().Signature
	err = textblock.Sign(sv)
	if err != nil {
		prev.SetError(err)
		return prev, err
	}
	err = prev.Add("claude", allThoughts, textblock, sv, allToolRequests, allToolReplies)
	if err != nil {
		prev.SetError(err)
		return prev, err
	}
	err = prev.Sign(sv)
	if err != nil {
		log.Printf("failed to sign Decision: %s", err)
	}
	if err == nil && !stateless {
		log.Printf("saving state to history")
		gp := prev.GenesisPrompt()
		p := gp.NextUnsigned(peerOutput.Describe(sv))
		err = p.Sign(sv)
		if err != nil {
			prev.SetError(err)
		} else {
			c.history.AppendInPlace(p, textblock)
		}
	}
	return prev, err
}
