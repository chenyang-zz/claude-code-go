package tool

import (
	"strings"
	"testing"
)

// testSchemaInput captures the typed input used by schema decode tests.
type testSchemaInput struct {
	Path      string   `json:"path"`
	Limit     int      `json:"limit"`
	Recursive bool     `json:"recursive"`
	Patterns  []string `json:"patterns"`
}

// TestDecodeInput verifies required fields, scalar types, and array items decode into a typed struct.
func TestDecodeInput(t *testing.T) {
	t.Parallel()

	schema := InputSchema{
		Properties: map[string]FieldSchema{
			"path": {
				Type:     ValueKindString,
				Required: true,
			},
			"limit": {
				Type:     ValueKindInteger,
				Required: true,
			},
			"recursive": {
				Type: ValueKindBoolean,
			},
			"patterns": {
				Type: ValueKindArray,
				Items: &FieldSchema{
					Type: ValueKindString,
				},
			},
		},
	}

	input, err := DecodeInput[testSchemaInput](schema, map[string]any{
		"path":      "src",
		"limit":     20.0,
		"recursive": true,
		"patterns":  []any{"*.go", "*.md"},
	})
	if err != nil {
		t.Fatalf("DecodeInput() error = %v", err)
	}

	if input.Path != "src" {
		t.Fatalf("DecodeInput() path = %q, want %q", input.Path, "src")
	}
	if input.Limit != 20 {
		t.Fatalf("DecodeInput() limit = %d, want %d", input.Limit, 20)
	}
	if !input.Recursive {
		t.Fatalf("DecodeInput() recursive = %v, want true", input.Recursive)
	}
	if len(input.Patterns) != 2 {
		t.Fatalf("DecodeInput() patterns len = %d, want %d", len(input.Patterns), 2)
	}
}

// TestInputSchemaDecodeMissingRequiredField verifies validation rejects missing required input.
func TestInputSchemaDecodeMissingRequiredField(t *testing.T) {
	t.Parallel()

	schema := InputSchema{
		Properties: map[string]FieldSchema{
			"path": {
				Type:     ValueKindString,
				Required: true,
			},
		},
	}

	_, err := DecodeInput[testSchemaInput](schema, map[string]any{})
	if err == nil {
		t.Fatal("DecodeInput() error = nil, want missing required field error")
	}
	if !strings.Contains(err.Error(), `missing required field "path"`) {
		t.Fatalf("DecodeInput() error = %v, want missing required field message", err)
	}
}

// TestInputSchemaDecodeUnexpectedField verifies undeclared keys are rejected before decoding.
func TestInputSchemaDecodeUnexpectedField(t *testing.T) {
	t.Parallel()

	schema := InputSchema{
		Properties: map[string]FieldSchema{
			"path": {
				Type: ValueKindString,
			},
		},
	}

	_, err := DecodeInput[testSchemaInput](schema, map[string]any{
		"path":  "src",
		"extra": true,
	})
	if err == nil {
		t.Fatal("DecodeInput() error = nil, want unexpected field error")
	}
	if !strings.Contains(err.Error(), `unexpected field "extra"`) {
		t.Fatalf("DecodeInput() error = %v, want unexpected field message", err)
	}
}

// TestInputSchemaDecodeTypeMismatch verifies field type validation fails with a stable message.
func TestInputSchemaDecodeTypeMismatch(t *testing.T) {
	t.Parallel()

	schema := InputSchema{
		Properties: map[string]FieldSchema{
			"limit": {
				Type: ValueKindInteger,
			},
		},
	}

	_, err := DecodeInput[testSchemaInput](schema, map[string]any{
		"limit": "ten",
	})
	if err == nil {
		t.Fatal("DecodeInput() error = nil, want type mismatch error")
	}
	if !strings.Contains(err.Error(), `field "limit" must be an integer`) {
		t.Fatalf("DecodeInput() error = %v, want integer type mismatch message", err)
	}
}

// TestInputSchemaDecodeArrayItemTypeMismatch verifies nested array item validation remains precise.
func TestInputSchemaDecodeArrayItemTypeMismatch(t *testing.T) {
	t.Parallel()

	schema := InputSchema{
		Properties: map[string]FieldSchema{
			"patterns": {
				Type: ValueKindArray,
				Items: &FieldSchema{
					Type: ValueKindString,
				},
			},
		},
	}

	_, err := DecodeInput[testSchemaInput](schema, map[string]any{
		"patterns": []any{"*.go", 3},
	})
	if err == nil {
		t.Fatal("DecodeInput() error = nil, want array item type mismatch error")
	}
	if !strings.Contains(err.Error(), `field "patterns[1]" must be a string`) {
		t.Fatalf("DecodeInput() error = %v, want nested array item mismatch message", err)
	}
}
