package brain

import (
	"context"

	"github.com/getkin/kin-openapi/openapi3"
)

func GenerateDreamPrompt(ctx context.Context, spec *openapi3.T, sv SignVerifier) (Signed, error) {
	prompt := "generate a JSON document that conforms to the following openapi component specification that describes the current context of the session. "

	if hydration, ok := spec.Components.Schemas["Hydration"]; ok {
		// Output as 'Manual Artifice' (JSON/YAML)
		FlattenSchema(hydration)
		data, _ := hydration.MarshalJSON()
		prompt += string(data)
	}
	p := NewUnsigned(prompt, TypePrompt)
	return p, p.Sign(sv)
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
