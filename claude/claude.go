package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/weberr13/ProjectIolite/brain"
)

type Claude struct {
	cl *anthropic.Client
}

type Option func(b *Claude)

func New(apiKey string, opts ...Option) (*Claude, error) {
	c := &Claude{}
	for _, o := range opts {
		o(c)
	}
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)
	c.cl = &client
	return c, nil
}

// Think generates the initial response
func (c *Claude) Think(ctx context.Context, sv brain.SignVerifier, input brain.Request) (brain.Response, error) {
	now := time.Now()
	defer func() {
		log.Printf("Think took: %v", time.Since(now))
	}()
	prompt := brain.NewUnsigned(input.Text(), "prompt")
	err := prompt.Sign(sv)
	if err != nil {
		return &ClaudeError{e: err}, err
	}
	message, err := c.cl.Messages.New(ctx, anthropic.MessageNewParams{
		MaxTokens: 1024 * 4,
		Thinking: anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{
				BudgetTokens: 1024,
				Type:         "enabled",
			},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(input.Text())),
		},
		Model: anthropic.ModelClaudeOpus4_6,
	})
	if err != nil {
		return &ClaudeError{e: err}, err
	}

	resp := &ClaudeResponse{resp: message, prompt: prompt, model: string(message.Model)}
	err = resp.Sign(sv)
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

// Add these to your imports: "os/exec", "bytes", "strings", "fmt"

func executePython(tooluse anthropic.ToolUseBlock) (string, error) {
	// 1. Extract the code from Claude's ToolUse input
	// The Input field is a raw JSON message from the SDK
	var input struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(tooluse.Input, &input); err != nil {
		return "", fmt.Errorf("failed to parse tool input: %w", err)
	}

	// 2. Prepare the python3 command
	// We use STDIN to avoid shell escaping issues with long scripts
	cmd := exec.Command("python3", "-")
	cmd.Stdin = strings.NewReader(input.Code)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 3. Fire the Piston
	err := cmd.Run()
	if err != nil {
		// We return the error string as the tool result so Claude can "see"
		// the traceback and try to fix its own code.
		return fmt.Sprintf("PYTHON ERROR:\n%s\n%s", stderr.String(), err.Error()), nil
	}

	return stdout.String(), nil
}

// Evaluate audits another brain's output
func (c *Claude) Evaluate(ctx context.Context, sv brain.SignVerifier, peerOutput brain.Response, prev brain.Decision) (brain.Decision, error) {
	now := time.Now()
	defer func() {
		log.Printf("Evaluate took: %v", time.Since(now))
	}()
	log.Printf("evaluating peer output %s", peerOutput.Describe(sv))
	if prev != nil {
		s, _ := json.MarshalIndent(prev, " ", " ")
		log.Printf("adding to existing Decision %s", s)
	}
	instruction := "You are the Iolite auditor. Your primary duty is to verify the [IOLITE_AUDIT_MANIFEST]." +
		"CRITICAL: Use the provided python_interpreter for verification. You must pass the Data_B64 strings exactly as they appear in the manifest into the data_b64 parameter of the verify_iolite_block function." +
		"Do not use the plaintext for cryptographic verification; it is for BTU evaluation only." +
		"If a block's Prev_Sig is present, you must pass that Base64 string into the prev_sig_b64 parameter to maintain the chain integrity." +
		"Report the True/False result for each block. once verifyied move on to BTU evaluation of the content text. " +
		"You must not trust the Data field in the JSON. You must verify the Data_B64 in the manifest. Once verified, you must Base64-decode the manifest yourself (using the python_interpreter or internally) and evaluate the resulting text. If the decoded text differs from the JSON text, the block is TAMPERED." +
		"POST-VERIFICATION: Once the signatures are verified (or fail), you MUST provide a full BTU evaluation. " +
		"Do not just report the math. Synthesis the given proposal offer the counter-perspective. " +
		"Your final response MUST be a complete analytical report, concluding with the approved: true/false JSON." +
		"Script: " + sv.VerifyPy()
	log.Printf("using model instructions: %s", instruction)

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

	message, err := c.cl.Messages.New(ctx, params)
	if err != nil {
		return &brain.ErrorDecision{E: err}, err
	}
	s, _ := json.MarshalIndent(message, " ", " ")
	log.Printf("got result: %s", s)
	maxRetries := 10 // should this be limited by the number of the checksums?
	i := 0
	var allThoughts []brain.Signed // <--- The Reservoir
	for i = range maxRetries {
		if message == nil {
			break
		}
		// 1. Capture any thinking from THIS turn
		turnThoughts := candidatesToThoughts(message)
		allThoughts = append(allThoughts, turnThoughts...)

		if message.StopReason != "tool_use" {
			log.Printf("done with tool use on iteration %d", i)
			break
		}
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
				log.Printf("tool request: %#v", toolUse)
				result, err := executePython(toolUse)
				isErr := false
				if err != nil {
					isErr = true
					result = err.Error()
					log.Printf("error executing tool: %s", err)
				}
				toolResults = append(toolResults, anthropic.NewToolResultBlock(toolUse.ID, result, isErr))
			}
		}
		if len(toolResults) > 0 {
			params.Messages = append(params.Messages, anthropic.NewUserMessage(toolResults...))
		}
		message, err = c.cl.Messages.New(ctx, params)
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
	allThoughts = append(allThoughts, candidatesToThoughts(message)...)
	if prev == nil {
		log.Printf("creating new decision struct")
		prev, err = NewDecision(peerOutput, sv)
		if err != nil {
			log.Printf("error creating decision struct")
			return nil, err
		}
	}
	textBlock := brain.NewUnsigned(extractText(message), "text")
	// STITCHING: Link Claude's audit to peer's Text Response
	textBlock.PrevSignature = peerOutput.Text().Signature

	err = prev.Add("claude", allThoughts, textBlock, sv)
	if err != nil {
		return prev, err
	}
	err = prev.Sign(sv)
	if err != nil {
		log.Printf("failed to sign Decision: %s", err)
	}
	return prev, err
}
