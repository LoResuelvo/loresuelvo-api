package evals

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var ErrExternalSchema = errors.New("external schema resolution is disabled")

type offlineSchemaLoader struct{}

func (offlineSchemaLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("%w: %s", ErrExternalSchema, url)
}
func compileEvaluationSchema(files map[string][]byte, name string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	compiler.UseLoader(offlineSchemaLoader{})
	raw, ok := files["schemas/"+name+".schema.json"]
	if !ok {
		return nil, fmt.Errorf("%w: missing schema %s", ErrInvalidDataset, name)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode schema %s: %w", name, err)
	}
	uri := "https://evals.invalid/" + name + ".schema.json"
	if err = compiler.AddResource(uri, document); err != nil {
		return nil, fmt.Errorf("add schema %s: %w", name, err)
	}
	schema, err := compiler.Compile(uri)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", name, err)
	}
	return schema, nil
}
func validateEvaluationOutput(dataset *Dataset, task string, raw json.RawMessage) error {
	schema, err := compileEvaluationSchema(dataset.schemaFiles, task+"-output")
	if err != nil {
		return err
	}
	output, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("decode output: %w", err)
	}
	return schema.Validate(output)
}

// ValidateDatasetSchemas validates frozen records with local Draft 2020-12 schemas.
// The caller supplies the same bytes that passed manifest verification.
func ValidateDatasetSchemas(files map[string][]byte) error {
	for _, name := range []string{"prediagnosis", "ranking", "service_contracts"} {
		schema, err := compileEvaluationSchema(files, name)
		if err != nil {
			return err
		}
		raw, ok := files["datasets/"+name+".jsonl"]
		if !ok {
			return fmt.Errorf("%w: missing records %s", ErrInvalidDataset, name)
		}
		for i, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
			record, decodeErr := jsonschema.UnmarshalJSON(bytes.NewReader(line))
			if decodeErr != nil {
				return fmt.Errorf("decode %s line %d: %w", name, i+1, decodeErr)
			}
			if err = schema.Validate(record); err != nil {
				return fmt.Errorf("validate %s line %d: %w", name, i+1, err)
			}
		}
	}
	return nil
}
