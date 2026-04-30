package config_tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/sheepzhao/claude-code-go/internal/platform/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

const Name = "Config"

// Tool implements the Config tool for reading and writing Claude Code configuration
// settings at runtime. It supports get (value omitted) and set (value provided)
// operations against a static registry of supported settings.
type Tool struct {
	writer *config.SettingsWriter
}

// NewTool creates a Config tool that reads and writes settings across
// user, project, and local scopes.
func NewTool(homeDir, projectDir string) *Tool {
	return &Tool{writer: config.NewSettingsWriter(homeDir, projectDir)}
}

func (t *Tool) Name() string { return Name }

func (t *Tool) Description() string {
	return "Get or set Claude Code configuration settings. " +
		"Provide a setting key to read its current value, or include a value to update it. " +
		"Supported settings: model, theme, editorMode, fastMode, effortLevel, permissions.defaultMode."
}

// Input defines the Config tool input schema.
type Input struct {
	Setting string `json:"setting"`
	Value   any    `json:"value,omitempty"`
}

// Output defines the Config tool output.
type Output struct {
	Success       bool   `json:"success"`
	Operation     string `json:"operation,omitempty"`
	Setting       string `json:"setting,omitempty"`
	Value         any    `json:"value,omitempty"`
	PreviousValue any    `json:"previousValue,omitempty"`
	NewValue      any    `json:"newValue,omitempty"`
	Error         string `json:"error,omitempty"`
}

var inputSchema = coretool.InputSchema{
	Properties: map[string]coretool.FieldSchema{
		"setting": {
			Type:        coretool.ValueKindString,
			Description: "The setting key (e.g., \"theme\", \"model\", \"permissions.defaultMode\")",
			Required:    true,
		},
		"value": {
			Type:        coretool.ValueKindString,
			Description: "The new value to set. Omit to read the current value. Accepted types: string, boolean, number.",
			Required:    false,
		},
	},
}

func (t *Tool) InputSchema() coretool.InputSchema {
	return inputSchema
}

func (t *Tool) IsReadOnly() bool        { return false }
func (t *Tool) IsConcurrencySafe() bool { return true }

// Invoke handles both get (value omitted) and set (value provided) operations.
func (t *Tool) Invoke(ctx context.Context, call coretool.Call) (coretool.Result, error) {
	if t.writer == nil {
		return coretool.Result{Error: "settings writer is not configured"}, nil
	}

	input, err := coretool.DecodeInput[Input](inputSchema, call.Input)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	setting := strings.TrimSpace(input.Setting)
	if setting == "" {
		return coretool.Result{Error: "setting key is required"}, nil
	}

	cfg, ok := getSupportedSetting(setting)
	if !ok {
		return coretool.Result{Error: fmt.Sprintf("Unknown setting: %q", setting)}, nil
	}

	// GET operation: value not provided
	if input.Value == nil {
		current, err := t.writer.Get(ctx, cfg.Scope, setting)
		if err != nil {
			return coretool.Result{Error: fmt.Sprintf("Failed to read setting: %s", err.Error())}, nil
		}
		return outputResult(Output{
			Success:   true,
			Operation: "get",
			Setting:   setting,
			Value:     normalizeForOutput(current, cfg.Type),
		})
	}

	// SET operation
	finalValue, err := coerceAndValidate(input.Value, cfg, setting)
	if err != nil {
		return coretool.Result{Error: err.Error()}, nil
	}

	// Read previous value before overwriting
	prevValue, _ := t.writer.Get(ctx, cfg.Scope, setting)

	if err := t.writer.Set(ctx, cfg.Scope, setting, finalValue); err != nil {
		logger.ErrorCF("config_tool", "failed to write setting", map[string]any{
			"setting": setting,
			"scope":   cfg.Scope,
			"error":   err.Error(),
		})
		return coretool.Result{Error: fmt.Sprintf("Failed to write setting: %s", err.Error())}, nil
	}

	logger.DebugCF("config_tool", "setting updated", map[string]any{
		"setting": setting,
		"scope":   cfg.Scope,
	})
	return outputResult(Output{
		Success:       true,
		Operation:     "set",
		Setting:       setting,
		NewValue:      normalizeForOutput(finalValue, cfg.Type),
		PreviousValue: normalizeForOutput(prevValue, cfg.Type),
	})
}

// coerceAndValidate applies type coercion, boolean parsing, and options validation.
func coerceAndValidate(raw any, cfg SettingConfig, setting string) (any, error) {
	finalValue := raw

	// Boolean coercion: accept string "true" / "false" as boolean values
	if cfg.Type == "boolean" {
		if strVal, ok := raw.(string); ok {
			lower := strings.ToLower(strings.TrimSpace(strVal))
			switch lower {
			case "true":
				finalValue = true
			case "false":
				finalValue = false
			}
		}
		if _, ok := finalValue.(bool); !ok {
			return nil, fmt.Errorf("%s requires true or false", setting)
		}
	}

	// Options validation: ensure the value is in the allowed set
	if len(cfg.Options) > 0 {
		strVal := fmt.Sprintf("%v", finalValue)
		if !containsString(cfg.Options, strVal) {
			return nil, fmt.Errorf("Invalid value %q. Options: %s", strVal, strings.Join(cfg.Options, ", "))
		}
	}

	return finalValue, nil
}

// outputResult serializes an Output value into a tool.Result.
func outputResult(o Output) (coretool.Result, error) {
	encoded, err := json.Marshal(o)
	if err != nil {
		return coretool.Result{Error: fmt.Sprintf("encode output: %s", err.Error())}, nil
	}
	return coretool.Result{Output: string(encoded)}, nil
}

// normalizeForOutput converts internal values to a stable JSON-friendly form.
func normalizeForOutput(v any, settingType string) any {
	if v == nil {
		return nil
	}
	switch settingType {
	case "boolean":
		b, ok := v.(bool)
		if ok {
			return b
		}
		return nil
	case "number":
		switch n := v.(type) {
		case float64:
			if n == float64(int64(n)) {
				return int64(n)
			}
			return n
		default:
			return v
		}
	default:
		// For string type, try to convert the value to a readable form
		switch val := v.(type) {
		case string:
			return val
		case float64:
			// Model names and other string settings may get decoded as float64 by JSON
			return strconv.FormatFloat(val, 'f', -1, 64)
		default:
			return fmt.Sprintf("%v", val)
		}
	}
}

// containsString returns true if the slice contains the target string.
func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
