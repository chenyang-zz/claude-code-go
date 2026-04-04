package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ValueKind describes the minimal JSON-compatible value types supported by tool input schemas.
type ValueKind string

const (
	// ValueKindString accepts string input values.
	ValueKindString ValueKind = "string"
	// ValueKindInteger accepts whole-number input values.
	ValueKindInteger ValueKind = "integer"
	// ValueKindNumber accepts numeric input values.
	ValueKindNumber ValueKind = "number"
	// ValueKindBoolean accepts boolean input values.
	ValueKindBoolean ValueKind = "boolean"
	// ValueKindObject accepts object-like input values.
	ValueKindObject ValueKind = "object"
	// ValueKindArray accepts array-like input values.
	ValueKindArray ValueKind = "array"
)

// FieldSchema describes one accepted top-level or nested tool input field.
type FieldSchema struct {
	// Type declares the expected JSON-compatible value shape.
	Type ValueKind
	// Description stores a short human-readable explanation for the field.
	Description string
	// Required marks whether the field must be present in the incoming payload.
	Required bool
	// Items describes the element shape for array-valued fields.
	Items *FieldSchema
}

// InputSchema defines the minimal object schema used to validate and decode tool inputs.
type InputSchema struct {
	// Properties enumerates the allowed input keys and their expected shapes.
	Properties map[string]FieldSchema
}

// JSONSchema converts the minimal tool schema into an Anthropic-compatible JSON schema subset.
func (s InputSchema) JSONSchema() map[string]any {
	properties := make(map[string]any, len(s.Properties))
	required := make([]string, 0, len(s.Properties))

	for name, field := range s.Properties {
		properties[name] = field.jsonSchema()
		if field.Required {
			required = append(required, name)
		}
	}

	sort.Strings(required)

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// jsonSchema converts one field schema into its JSON-schema-like representation.
func (f FieldSchema) jsonSchema() map[string]any {
	schema := map[string]any{
		"type": string(f.Type),
	}
	if f.Description != "" {
		schema["description"] = f.Description
	}
	if f.Type == ValueKindArray && f.Items != nil {
		schema["items"] = f.Items.jsonSchema()
	}
	return schema
}

// DecodeInput validates the raw input map against the provided schema and decodes it into T.
func DecodeInput[T any](schema InputSchema, input map[string]any) (T, error) {
	var target T
	if err := schema.Decode(input, &target); err != nil {
		return target, err
	}
	return target, nil
}

// Decode validates the raw input map and decodes it into the provided target struct pointer.
func (s InputSchema) Decode(input map[string]any, target any) error {
	if target == nil {
		return fmt.Errorf("tool schema: decode target is nil")
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.IsNil() {
		return fmt.Errorf("tool schema: decode target must be a non-nil pointer")
	}

	normalizedInput := input
	if normalizedInput == nil {
		normalizedInput = map[string]any{}
	}

	logger.DebugCF("tool_schema", "decoding tool input", map[string]any{
		"field_count": len(normalizedInput),
	})

	if err := s.Validate(normalizedInput); err != nil {
		logger.DebugCF("tool_schema", "tool input validation failed", map[string]any{
			"error": err.Error(),
		})
		return err
	}

	raw, err := json.Marshal(normalizedInput)
	if err != nil {
		return fmt.Errorf("tool schema: marshal input: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		logger.DebugCF("tool_schema", "tool input decode failed", map[string]any{
			"error": err.Error(),
		})
		return fmt.Errorf("tool schema: decode input: %w", err)
	}

	logger.DebugCF("tool_schema", "tool input decoded", map[string]any{
		"field_count": len(normalizedInput),
	})
	return nil
}

// Validate ensures the raw input payload matches the declared schema before typed decoding.
func (s InputSchema) Validate(input map[string]any) error {
	normalizedInput := input
	if normalizedInput == nil {
		normalizedInput = map[string]any{}
	}

	if len(s.Properties) == 0 && len(normalizedInput) > 0 {
		return fmt.Errorf("tool schema: unexpected field %q", firstSortedKey(normalizedInput))
	}

	for name, field := range s.Properties {
		value, ok := normalizedInput[name]
		if field.Required && !ok {
			return fmt.Errorf("tool schema: missing required field %q", name)
		}
		if !ok {
			continue
		}
		if err := validateFieldValue(name, field, value); err != nil {
			return err
		}
	}

	for name := range normalizedInput {
		if _, exists := s.Properties[name]; !exists {
			return fmt.Errorf("tool schema: unexpected field %q", firstSortedUnknownKey(normalizedInput, s.Properties))
		}
	}

	return nil
}

// validateFieldValue enforces the declared shape for a single field value.
func validateFieldValue(path string, field FieldSchema, value any) error {
	switch field.Type {
	case ValueKindString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("tool schema: field %q must be a string", path)
		}
	case ValueKindInteger:
		if !isIntegerValue(value) {
			return fmt.Errorf("tool schema: field %q must be an integer", path)
		}
	case ValueKindNumber:
		if !isNumericValue(value) {
			return fmt.Errorf("tool schema: field %q must be a number", path)
		}
	case ValueKindBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("tool schema: field %q must be a boolean", path)
		}
	case ValueKindObject:
		if !isObjectValue(value) {
			return fmt.Errorf("tool schema: field %q must be an object", path)
		}
	case ValueKindArray:
		arrayValue, ok := asArray(value)
		if !ok {
			return fmt.Errorf("tool schema: field %q must be an array", path)
		}
		if field.Items == nil {
			return nil
		}
		for index, item := range arrayValue {
			if err := validateFieldValue(fmt.Sprintf("%s[%d]", path, index), *field.Items, item); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("tool schema: field %q uses unsupported type %q", path, field.Type)
	}

	return nil
}

// isIntegerValue reports whether v can safely be interpreted as an integer input.
func isIntegerValue(v any) bool {
	switch value := v.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0) && math.Trunc(float64(value)) == float64(value)
	case float64:
		return !math.IsNaN(value) && !math.IsInf(value, 0) && math.Trunc(value) == value
	default:
		return false
	}
}

// isNumericValue reports whether v is any supported numeric input type.
func isNumericValue(v any) bool {
	switch value := v.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
	case float64:
		return !math.IsNaN(value) && !math.IsInf(value, 0)
	default:
		return false
	}
}

// isObjectValue reports whether v is a map-like value with string keys.
func isObjectValue(v any) bool {
	if _, ok := v.(map[string]any); ok {
		return true
	}

	value := reflect.ValueOf(v)
	return value.IsValid() && value.Kind() == reflect.Map && value.Type().Key().Kind() == reflect.String
}

// asArray converts any slice or array input into a generic item slice for validation.
func asArray(v any) ([]any, bool) {
	if items, ok := v.([]any); ok {
		return items, true
	}

	value := reflect.ValueOf(v)
	if !value.IsValid() {
		return nil, false
	}
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil, false
	}

	items := make([]any, value.Len())
	for i := 0; i < value.Len(); i++ {
		items[i] = value.Index(i).Interface()
	}
	return items, true
}

// firstSortedKey returns the lexicographically smallest key for deterministic error messages.
func firstSortedKey(values map[string]any) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}

// firstSortedUnknownKey returns the lexicographically smallest key that is not declared in the schema.
func firstSortedUnknownKey(values map[string]any, allowed map[string]FieldSchema) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if _, ok := allowed[key]; ok {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}
