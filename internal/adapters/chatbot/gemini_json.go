package chatbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// decodeStrictJSON applies the structural guarantees that the provider schema
// requests again at the adapter boundary. Provider-side structured output is a
// generation aid, not a replacement for validating untrusted output locally.
func decodeStrictJSON(raw string, responseSchema map[string]any, target any) error {
	trimmed := strings.TrimSpace(raw)
	if err := rejectDuplicateJSONFields(trimmed); err != nil {
		return err
	}
	if err := validateJSONAgainstSchema(trimmed, responseSchema); err != nil {
		return err
	}

	decoder := json.NewDecoder(strings.NewReader(trimmed))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := requireJSONEOF(decoder); err != nil {
		return err
	}
	return nil
}

func validateJSONAgainstSchema(raw string, responseSchema map[string]any) error {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	const schemaLocation = "https://chatbot.invalid/response.schema.json"
	encodedSchema, err := json.Marshal(responseSchema)
	if err != nil {
		return fmt.Errorf("encoding response JSON schema: %w", err)
	}
	schemaDocument, err := jsonschema.UnmarshalJSON(bytes.NewReader(encodedSchema))
	if err != nil {
		return fmt.Errorf("decoding response JSON schema: %w", err)
	}
	if err := compiler.AddResource(schemaLocation, schemaDocument); err != nil {
		return fmt.Errorf("adding response JSON schema: %w", err)
	}
	compiled, err := compiler.Compile(schemaLocation)
	if err != nil {
		return fmt.Errorf("compiling response JSON schema: %w", err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader([]byte(raw)))
	if err != nil {
		return err
	}
	return compiled.Validate(document)
}

func rejectDuplicateJSONFields(raw string) error {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, 0); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func scanJSONValue(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return fmt.Errorf("JSON nesting exceeds maximum depth")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := nameToken.(string)
			if !ok {
				return fmt.Errorf("JSON object field name must be a string")
			}
			if _, duplicate := seen[name]; duplicate {
				return fmt.Errorf("duplicate JSON field %q", name)
			}
			seen[name] = struct{}{}
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}

	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	expected := json.Delim('}')
	if delimiter == '[' {
		expected = ']'
	}
	if closing != expected {
		return fmt.Errorf("unexpected JSON delimiter %q", closing)
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}
