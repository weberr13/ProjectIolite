package brain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

var (
	ErrInvalidSchema    = errors.New("cannot extract a Hydration object schema from the api spec")
	ErrNoHydrationFound = errors.New("no valid hydration document found")
)

type SignedHydration struct {
	PublicKey string `json:"PublicKey"`
	Hydration
	Signed
}

func (e *SignedHydration) UnmarshalJSON(b []byte) error {
	type Alias SignedHydration
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	m := map[string]any{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}
	e.PublicKey = m["PublicKey"].(string)
	return nil
}

type Hydration struct {
	Timestamp            time.Time         `json:"timestamp"`
	Subject              string            `json:"subject,omitempty"`
	ContextOrigin        string            `json:"context_origin"`
	MigrationID          string            `json:"migration_id,omitempty"`
	ActiveHeuristics     map[string]string `json:"active_heuristics"`
	ForensicMilestones   map[string]string `json:"forensic_milestones"`
	TechnicalBenchmarks  map[string]string `json:"technical_benchmarks"`
	PhilosophicalAnchors map[string]string `json:"philosophical_anchors"`
	InstructionOverride  string            `json:"instruction_override,omitempty"`
}

func (e *SignedHydration) GetPublicKey() string {
	return e.PublicKey
}

func (sh *SignedHydration) Sign(sv SignVerifier) error {
	b, err := json.Marshal(sh.Hydration)
	if err != nil {
		return err
	}
	sh.Signed.Data = string(b)
	sh.PublicKey = sv.ExportPublicKey()
	err = sh.Signed.Sign(sv)
	if err != nil {
		return err
	}
	return err
}

func (sh *SignedHydration) Verify(sv SignVerifier) error {
	if sh.Signature == "" {
		log.Printf("UNSIGNED: %#v", sh)
		return ErrUnsigned
	}
	if sh.Signed.Data64 == "" {
		log.Printf("reconstituting data")
		b, err := json.Marshal(sh.Hydration)
		if err != nil {
			return err
		}
		sh.Signed.Data = string(b)
		sh.Signed.Data64 = base64.StdEncoding.EncodeToString([]byte(sh.Signed.Data))
	}
	if sh.PublicKey != "" {
		log.Printf("using given public key: %s", sh.PublicKey)
		return sv.Verify(sh.Signed.Data64+sh.Signed.PrevSignature, sh.Signed.Signature, sh.PublicKey)
	}
	return sv.Verify(sh.Signed.Data64+sh.Signed.PrevSignature, sh.Signed.Signature)
}

func (d *SignedHydration) Blocks() []*Signed {
	return []*Signed{&d.Signed}
}

func GenerateChimePrompt(ctx context.Context, sv SignVerifier, h Chimetric) (Signed, error) {
	prompt := "utilize the following document to integrate domain specific information your session context:"
	prompt += "```json"
	b, err := json.Marshal(h)
	if err != nil {
		return Signed{}, err
	}
	prompt += string(b)
	prompt += "```"
	p := NewUnsigned(prompt, TypePrompt)
	return p, p.Sign(sv)
}

func GenerateHyrationPrompt(ctx context.Context, sv SignVerifier, h SignedHydration) (Signed, error) {
	prompt := "utilize the following document to hydrate your session context with a summary of a previous context:"
	prompt += "```json"
	b, err := json.Marshal(h)
	if err != nil {
		return Signed{}, err
	}
	prompt += string(b)
	prompt += "```"
	p := NewUnsigned(prompt, TypePrompt)
	return p, p.Sign(sv)
}

func ParseChimeReponse(ctx context.Context, sv SignVerifier, text string) error {
	log.Printf("got chime response: %s\n", text)
	return nil
}

func ParseHydrationReponse(ctx context.Context, sv SignVerifier, text string) error {
	log.Printf("got wake response: %s\n", text)
	return nil
}

func ParseLacunaReponse(ctx context.Context, sv SignVerifier, text string) error {
	log.Printf("got lacuna response: %s\n", text)
	return nil
}

func GenerateLacunaPrompt(ctx context.Context, sv SignVerifier, l Lacuna) (Signed, error) {
	prompt := "utilize the following document to define a lacuna, a new word or domain expansion of an existing word:"
	prompt += "```json"
	b, err := json.Marshal(l.LexiconAugmentation)
	if err != nil {
		return Signed{}, err
	}
	prompt += string(b)
	prompt += "```"
	p := NewUnsigned(prompt, TypePrompt)
	return p, p.Sign(sv)
}

func GenerateDreamPrompt(ctx context.Context, spec *openapi3.T, sv SignVerifier) (Signed, error) {
	prompt := "generate a JSON document that conforms to the following openapi component specification that describes the current context of the session and includes at least 6 elements in each of the active_heuristics, technical_benchmarks, forensic_milestones, philosophical_anchors sections. "

	if hydration, ok := spec.Components.Schemas["Hydration"]; ok {
		FlattenSchema(hydration)
		data, _ := hydration.MarshalJSON()
		prompt += string(data)
	}
	p := NewUnsigned(prompt, TypePrompt)
	return p, p.Sign(sv)
}

func ExtractBalancedJSON(input string) []string {
	var results []string
	start := -1
	balance := 0
	inString := false

	for i, char := range input {
		// Toggle string state to ignore braces inside quotes
		if char == '"' && (i == 0 || input[i-1] != '\\') {
			inString = !inString
		}

		if !inString {
			switch char {
			case '{':
				if balance == 0 {
					start = i
				}
				balance++
			case '}':
				balance--
				if balance == 0 && start != -1 {
					results = append(results, input[start:i+1])
					start = -1
				}
			}
		}
	}
	return results
}

func ParseDreamResponse(ctx context.Context, spec *openapi3.T, sv SignVerifier, now time.Time, input string) ([]SignedHydration, error) {
	hydration, ok := spec.Components.Schemas["Hydration"]
	if !ok {
		return nil, ErrInvalidSchema
	}
	FlattenSchema(hydration)
	data, _ := hydration.MarshalJSON()
	m := map[string]any{}
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, err
	}
	required, ok := m["required"].([]any)
	if !ok {
		return nil, ErrInvalidSchema
	}
	requiredKeys := make([]string, 0, len(m))
	for _, k := range required {
		ks, ok := k.(string)
		if !ok {
			return nil, ErrInvalidSchema
		}
		requiredKeys = append(requiredKeys, ks)
	}

	candidates := ExtractBalancedJSON(input)
	hydrations := []string{}
	for _, c := range candidates {
		matchCount := 0
		for _, key := range requiredKeys {
			if strings.Contains(c, `"`+key+`":`) {
				matchCount++
			}
		}

		// 3. Assertion: All keys must exist within the block
		if matchCount == len(requiredKeys) {
			// Check for balanced braces as a basic sanity gate before heavy unmarshalling
			if strings.Count(c, "{") == strings.Count(c, "}") {
				hydrations = append(hydrations, c)
			}
		}
	}
	if len(hydrations) == 0 {
		return nil, ErrNoHydrationFound
	}
	shs := make([]SignedHydration, 0, len(hydrations))
	for i, h := range hydrations {
		base := Hydration{}
		err := json.Unmarshal([]byte(h), &base)
		if err != nil {
			if i == len(hydrations)-1 && len(shs) == 0 { // last chance!
				return nil, err
			}
			continue
		}
		base.Timestamp = now.UTC() // LLMs don't undrestand time, these values are hallucinations
		sh := SignedHydration{
			Signed:    NewUnsigned(h, TypeHydration),
			Hydration: base,
		}
		err = sh.Sign(sv)
		if err != nil {
			return nil, err // return immediately if corrupted data is found
		}
		shs = append(shs, sh)
	}
	if len(shs) == 0 {
		return nil, ErrNoHydrationFound
	}
	return shs, nil
}

// FlattenSchema recursively clears Ref strings to force Value serialization.
func FlattenSchema(s *openapi3.SchemaRef) {
	if s == nil {
		return
	}
	// [CRITICAL]: Clear the Ref string to force MarshalJSON to use the Value.
	s.Ref = ""

	if s.Value == nil {
		return
	}

	s.Value.Example = nil // examples will inadvertently anchor the model towards the spec
	// Recurse through properties
	for _, prop := range s.Value.Properties {
		FlattenSchema(prop)
	}

	// Recurse through arrays/items
	if s.Value.Items != nil {
		FlattenSchema(s.Value.Items)
	}

	// Recurse through maps/additionalProperties
	if s.Value.AdditionalProperties.Schema != nil {
		FlattenSchema(s.Value.AdditionalProperties.Schema)
	}

	// Recurse through combinators (allOf, anyOf, oneOf)
	for _, sub := range s.Value.AllOf {
		FlattenSchema(sub)
	}
	for _, sub := range s.Value.AnyOf {
		FlattenSchema(sub)
	}
	for _, sub := range s.Value.OneOf {
		FlattenSchema(sub)
	}
}
