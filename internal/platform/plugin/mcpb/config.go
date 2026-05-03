package mcpb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// LoadMcpServerUserConfig loads user configuration for an MCP server by
// reading from the plugin config store. Non-sensitive values come from
// settings.json pluginConfigs; sensitive values are loaded from
// secure storage via the provided sensitiveLoader function.
//
// The sensitiveLoader function receives a composite key "pluginId/serverName"
// and should return the stored key-value pairs, or nil if none exist.
//
// Returns nil only if neither source has any data.
func LoadMcpServerUserConfig(
	pluginID string,
	serverName string,
	nonSensitive UserConfigValues,
	sensitiveLoader func(key string) UserConfigValues,
) UserConfigValues {
	merged := make(UserConfigValues)

	// Load non-sensitive values first.
	if nonSensitive != nil {
		for k, v := range nonSensitive {
			merged[k] = v
		}
	}

	// Load sensitive values (win on collision).
	if sensitiveLoader != nil {
		sensitiveKey := serverSecretsKey(pluginID, serverName)
		if sensitive := sensitiveLoader(sensitiveKey); sensitive != nil {
			for k, v := range sensitive {
				merged[k] = v
			}
		}
	}

	if len(merged) == 0 {
		return nil
	}

	logger.DebugCF("plugin.mcpb", "loaded user config", map[string]any{
		"pluginId":   pluginID,
		"serverName": serverName,
		"keys":       len(merged),
	})

	return merged
}

// SaveMcpServerUserConfig splits user configuration by schema[key].Sensitive
// and persists to the two backing stores. The nonSensitiveSaver is called with
// non-sensitive values to write to settings.json; the sensitiveSaver is called
// with sensitive values for secure storage.
func SaveMcpServerUserConfig(
	pluginID string,
	serverName string,
	config UserConfigValues,
	schema map[string]McpbConfigOption,
	nonSensitiveSaver func(values UserConfigValues) error,
	sensitiveSaver func(key string, values UserConfigValues) error,
) error {
	nonSensitive := make(UserConfigValues)
	sensitive := make(UserConfigValues)

	for key, value := range config {
		opt, exists := schema[key]
		if exists && opt.Sensitive {
			sensitive[key] = fmt.Sprintf("%v", value)
		} else {
			nonSensitive[key] = value
		}
	}

	// Write sensitive values first. If this fails, the old plaintext copy
	// remains in settings.json as a fallback.
	if len(sensitive) > 0 && sensitiveSaver != nil {
		sensitiveKey := serverSecretsKey(pluginID, serverName)
		if err := sensitiveSaver(sensitiveKey, sensitive); err != nil {
			return fmt.Errorf("failed to save sensitive config to secure storage for %s/%s: %w",
				pluginID, serverName, err)
		}
	}

	// Write non-sensitive values to settings.json.
	if len(nonSensitive) > 0 && nonSensitiveSaver != nil {
		if err := nonSensitiveSaver(nonSensitive); err != nil {
			return fmt.Errorf("failed to save non-sensitive config for %s/%s: %w",
				pluginID, serverName, err)
		}
	}

	logger.DebugCF("plugin.mcpb", "saved user config", map[string]any{
		"pluginId":       pluginID,
		"serverName":     serverName,
		"nonSensitive":   len(nonSensitive),
		"sensitive":      len(sensitive),
	})

	return nil
}

// ValidateUserConfig checks user configuration values against a DXT
// user_config schema. It returns a list of validation error messages.
// An empty list means the configuration is valid.
func ValidateUserConfig(values UserConfigValues, schema map[string]McpbConfigOption) []string {
	var errors []string

	for key, field := range schema {
		value, hasValue := values[key]

		// Check required fields.
		if field.Required {
			if !hasValue || isEmptyValue(value) {
				label := field.Title
				if label == "" {
					label = key
				}
				errors = append(errors, fmt.Sprintf("%s is required but not provided", label))
				continue
			}
		}

		// Skip validation for optional fields that are not provided.
		if !hasValue || isEmptyValue(value) {
			continue
		}

		// Type validation.
		errors = append(errors, validateFieldType(key, value, field)...)
	}

	return errors
}

// isEmptyValue checks whether a value is empty for validation purposes.
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case []any:
		return len(val) == 0
	}
	return false
}

// validateFieldType checks the type of a single field value against the schema.
func validateFieldType(key string, value any, field McpbConfigOption) []string {
	var errors []string
	label := field.Title
	if label == "" {
		label = key
	}

	switch field.Type {
	case "string":
		if arr, ok := value.([]any); ok {
			if !field.Multiple {
				errors = append(errors, fmt.Sprintf("%s must be a string, not an array", label))
			} else {
				for _, v := range arr {
					if _, ok := v.(string); !ok {
						errors = append(errors, fmt.Sprintf("%s must be an array of strings", label))
						break
					}
				}
			}
		} else if _, ok := value.(string); !ok {
			errors = append(errors, fmt.Sprintf("%s must be a string", label))
		}
	case "number":
		if num, ok := toFloat64(value); ok {
			if field.Min != nil && num < *field.Min {
				errors = append(errors, fmt.Sprintf("%s must be at least %v", label, *field.Min))
			}
			if field.Max != nil && num > *field.Max {
				errors = append(errors, fmt.Sprintf("%s must be at most %v", label, *field.Max))
			}
		} else {
			errors = append(errors, fmt.Sprintf("%s must be a number", label))
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			errors = append(errors, fmt.Sprintf("%s must be a boolean", label))
		}
	case "file", "directory":
		if _, ok := value.(string); !ok {
			errors = append(errors, fmt.Sprintf("%s must be a path string", label))
		}
	}

	return errors
}

// toFloat64 attempts to convert a value to float64.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

// serverSecretsKey composes the secure storage key for per-server secrets.
func serverSecretsKey(pluginID, serverName string) string {
	return pluginID + "/" + serverName
}

// SplitMcpServerUserConfigPath splits a plugin config path into the portion
// relative to mcpServers for a given server name. For example, given
// "pluginConfigs.myPlugin.mcpServers.myServer.someKey", it returns the
// plugin ID and server name portions.
func SplitMcpServerUserConfigPath(key string) (pluginID, field string) {
	parts := strings.SplitN(key, ".", 3)
	if len(parts) >= 2 {
		return parts[1], parts[len(parts)-1]
	}
	return "", key
}
