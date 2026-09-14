package chatbot

import (
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
)

func answerResponseJSONSchema(titleRequired bool, newImageCount int, availableCategories []category.Category) map[string]any {
	titleSchema := stringJSONSchema()
	if titleRequired {
		titleSchema["minLength"] = 1
	}
	categoryNames := []string{""}
	seenCategories := map[string]struct{}{"": {}}
	for _, availableCategory := range availableCategories {
		name := strings.TrimSpace(availableCategory.Name)
		if _, seen := seenCategories[name]; name == "" || seen {
			continue
		}
		seenCategories[name] = struct{}{}
		categoryNames = append(categoryNames, name)
	}

	return objectJSONSchema(
		[]string{"status", "title", "content", "image_descriptions", "assessment"},
		map[string]any{
			"status":  enumStringJSONSchema("answered", "out_of_scope"),
			"title":   titleSchema,
			"content": nonEmptyStringJSONSchema(),
			"image_descriptions": map[string]any{
				"type":     "array",
				"minItems": newImageCount,
				"maxItems": newImageCount,
				"items": objectJSONSchema(
					[]string{"image_ref", "description"},
					map[string]any{
						"image_ref":   nonEmptyStringJSONSchema(),
						"description": nonEmptyStringJSONSchema(),
					},
				),
			},
			"assessment": objectJSONSchema(
				[]string{"action", "outcome", "problem_title", "problem_description", "problem_category_name", "selected_image_refs"},
				map[string]any{
					"action":                enumStringJSONSchema("unchanged", "replace"),
					"outcome":               enumStringJSONSchema("", "collecting_information", "self_service", "professional_required"),
					"problem_title":         stringJSONSchema(),
					"problem_description":   stringJSONSchema(),
					"problem_category_name": enumStringJSONSchema(categoryNames...),
					"selected_image_refs": map[string]any{
						"type":     "array",
						"maxItems": 3,
						"items":    nonEmptyStringJSONSchema(),
					},
				},
			),
		},
	)
}

func summaryResponseJSONSchema() map[string]any {
	return objectJSONSchema(
		[]string{"summary"},
		map[string]any{"summary": nonEmptyStringJSONSchema()},
	)
}

func providerRankingResponseJSONSchema(maxResults int) map[string]any {
	recommendations := map[string]any{
		"type": "array",
		"items": objectJSONSchema(
			[]string{"reference", "reason"},
			map[string]any{
				"reference": nonEmptyStringJSONSchema(),
				"reason":    nonEmptyStringJSONSchema(),
			},
		),
	}
	if maxResults >= 0 {
		recommendations["maxItems"] = maxResults
	}
	return objectJSONSchema(
		[]string{"recommendations"},
		map[string]any{"recommendations": recommendations},
	)
}

func objectJSONSchema(required []string, properties map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             required,
		"properties":           properties,
	}
}

func enumStringJSONSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func nonEmptyStringJSONSchema() map[string]any {
	return map[string]any{"type": "string", "minLength": 1}
}

func stringJSONSchema() map[string]any {
	return map[string]any{"type": "string"}
}
