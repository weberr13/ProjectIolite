package claude

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/weberr13/ProjectIolite/brain"
)

const (
	characterTruncationLimit = 400
)

type ClaudeResponse struct {
	resp    *anthropic.Message
	cot     []brain.Signed
	thought *brain.Signed
	model   string
	*brain.BaseResponse
}

func candidatesToThoughts(sv brain.SignVerifier, resp *anthropic.Message, prompt brain.Signed, includeText ...bool) ([]brain.Signed, error) {
	thoughts := []brain.Signed{}
	captureFromText := false
	if len(includeText) > 0 && includeText[0] {
		captureFromText = true
	}
	for _, c := range resp.Content {
		if captureFromText {
			if c.Type == "text" {
				last := len(thoughts) - 1
				if last < 0 {
					t := prompt.NextUnsigned(c.AsText().Text, brain.TypeThinking)
					t.IsShadow = true
					err := t.Sign(sv)
					if err != nil {
						return nil, err
					}
					thoughts = append(thoughts, t)
				} else {
					t := thoughts[last].NextUnsigned(c.AsText().Text)
					t.IsShadow = true
					err := t.Sign(sv)
					if err != nil {
						return nil, err
					}
					thoughts = append(thoughts, t)
				}
			}
			continue
		}
		if c.Type == "thinking" || c.Type == "redacted_thinking" { // What is redacted thinking????
			last := len(thoughts) - 1
			if last < 0 {
				t := prompt.NextUnsigned(c.AsThinking().Thinking, brain.TypeThinking)
				err := t.Sign(sv)
				if err != nil {
					return nil, err
				}
				thoughts = append(thoughts, t)
			} else {
				t := thoughts[last].NextUnsigned(c.AsThinking().Thinking)
				err := t.Sign(sv)
				if err != nil {
					return nil, err
				}
				thoughts = append(thoughts, t)
			}
		}
	}
	return thoughts, nil
}

func (r *ClaudeResponse) CoT(sv brain.SignVerifier, quiet ...bool) []brain.Signed {
	if r.cot == nil {
		cot, err := candidatesToThoughts(sv, r.resp, r.Prompt())
		if err != nil {
			return nil
		}
		r.cot = cot
	}
	if len(quiet) > 0 && quiet[0] {
		c := make([]brain.Signed, 0, len(r.cot))
		for _, ct := range r.cot {
			ct.Data64 = ""
			c = append(c, ct)
		}
		return c
	}
	return r.cot
}

func (r *ClaudeResponse) Text() *brain.Signed {
	if r.thought == nil {
		s := brain.NewUnsigned(extractText(r.resp), brain.TypeText)
		s.PrevSignature = r.Prompt().Signature
		r.thought = &s
	}
	return r.thought
}

func (r *ClaudeResponse) Thought(quiet ...bool) brain.Signed {
	if len(quiet) > 0 && quiet[0] {
		t := *r.Text()
		t.Data64 = ""
		return t
	}
	return *r.Text()
}

func (r *ClaudeResponse) Sign(sv brain.SignVerifier) error {
	if r.cot == nil {
		_ = r.CoT(sv)
	}
	if r.thought == nil {
		_ = r.Text()
	}
	err := r.SignPrompt(sv)
	if err != nil {
		return err
	}
	for i := range r.cot {
		err = r.cot[i].Sign(sv)
		if err != nil {
			return err
		}
	}
	err = r.thought.Sign(sv)
	return err
}

func (r *ClaudeResponse) Verify(sv brain.SignVerifier) error {
	if r.cot == nil || r.thought == nil {
		log.Printf("UNSIGNED: %#v", r)
		return brain.ErrUnsigned
	}
	var err error
	for i := range r.cot {
		err = r.cot[i].Verify(sv)
		if err != nil {
			return err
		}
	}
	return r.thought.Verify(sv)
}

func objToString(v any) string {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false) // <--- THIS SAVES YOUR CREDITS
	_ = encoder.Encode(v)
	return strings.TrimSpace(buffer.String())
}

func (r *ClaudeResponse) Describe(sv brain.SignVerifier) string {
	// TODO: text formatter would be better than this
	var builder strings.Builder
	builder.WriteString("### [IOLITE_AUDIT_MANIFEST]\n")
	builder.WriteString(fmt.Sprintf("Public_Key: %s\n\n", sv.ExportPublicKey()))

	// TODO: move this to "brain" if possible
	// 1. Prompt Block
	builder.WriteString("#### Block: Prompt\n")
	builder.WriteString(fmt.Sprintf("- Signature: %s\n", r.Prompt().Signature))
	builder.WriteString(fmt.Sprintf("- Prev_Sig: %s\n", r.Prompt().PrevSignature))
	builder.WriteString(fmt.Sprintf("- Data_B64: %s\n\n", base64.StdEncoding.EncodeToString([]byte(r.Prompt().Data))))

	// 2. Text Block
	builder.WriteString("#### Block: Text_Response\n")
	b64 := base64.StdEncoding.EncodeToString([]byte(r.thought.Data))
	if len(b64) > characterTruncationLimit {
		builder.WriteString(fmt.Sprintf("- Verified_By_Agent: %v\n", r.thought.Verify(sv) == nil))
	} else {
		builder.WriteString(fmt.Sprintf("- Signature: %s\n", r.thought.Signature))
		builder.WriteString(fmt.Sprintf("- Prev_Sig: %s\n", r.thought.PrevSignature))
		builder.WriteString(fmt.Sprintf("- Data_B64: %s\n\n", base64.StdEncoding.EncodeToString([]byte(r.thought.Data))))
	}

	// 3. CoT Block
	builder.WriteString("#### Block: CoT_Response\n")
	for _, c := range r.cot {
		builder.WriteString(fmt.Sprintf("- Verified_By_Agent: %v\n", c.Verify(sv) == nil))
		// builder.WriteString(fmt.Sprintf("- Signature: %s\n", c.Signature))
		// builder.WriteString(fmt.Sprintf("- Prev_Sig: %s\n", c.PrevSignature))
		// builder.WriteString(fmt.Sprintf("- Data_B64: %s\n\n", base64.StdEncoding.EncodeToString([]byte(c.Data))))
	}

	builder.WriteString(`the base64 data is provided to you for lightweight verification but the agent bears sole responsibilty for signature verification and the data_B64 is truthful and no additional text verification is allowed (such as recoding text to base64)`)
	builder.WriteString("### [PLAINTEXT_FOR_BTU_EVALUATION]\n")
	builder.WriteString(`the following prompt and data is generated by Claude "`)
	builder.WriteString(r.model)
	builder.WriteString(`". the input prompt """`)
	builder.WriteString(objToString(r.Prompt(true)))
	builder.WriteString(`""" produced the response json """`)
	builder.WriteString(objToString(r.Thought(true)))
	builder.WriteString(`""" with chain of thought described by the following json array of signed responses: `)
	builder.WriteString(objToString(r.CoT(sv, true)))
	builder.WriteString("each signature in the 3 json objects above can be cryptograpically verified using ")
	builder.WriteString(sv.Alg())
	builder.WriteString(" and public key ")
	builder.WriteString(sv.ExportPublicKey())
	// TODO: Inject this into the "instructions for ... and not in every prompt"
	// builder.WriteString("you can use the following code snippet to verify the integrity of each signature")
	// builder.WriteString(brain.DecodePy)
	builder.WriteString(". Please evaluate this response based on the BTU protocols:")
	builder.WriteString(brain.BRAVE)
	builder.WriteString(brain.TRUTHFUL)
	builder.WriteString(brain.UNSELFISH)
	return builder.String()
}
