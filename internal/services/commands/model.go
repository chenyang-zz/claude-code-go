package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/sheepzhao/claude-code-go/internal/core/command"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	"github.com/sheepzhao/claude-code-go/pkg/logger"
)

// ModelSettingStore persists the global model preference for slash commands that mutate inference defaults.
type ModelSettingStore interface {
	// SaveModel writes the requested model override into durable user-scoped settings.
	SaveModel(ctx context.Context, model string) error
}

// ModelCommand exposes the minimum text-only /model flow before the interactive picker exists in the Go host.
type ModelCommand struct {
	// Config carries the resolved runtime configuration snapshot used to derive the current model.
	Config *coreconfig.Config
	// Store persists the updated model setting into global settings.
	Store ModelSettingStore
}

// Metadata returns the canonical slash descriptor for /model.
func (c ModelCommand) Metadata() command.Metadata {
	return command.Metadata{
		Name:        "model",
		Description: "Change the model",
		Usage:       "/model [model]",
	}
}

// Execute reports the current model or persists an explicit model override using the minimum text-only contract.
func (c ModelCommand) Execute(ctx context.Context, args command.Args) (command.Result, error) {
	currentModel := currentModelValue(c.Config)
	requested := strings.TrimSpace(args.RawLine)

	if requested == "" {
		return command.Result{
			Output: fmt.Sprintf(
				"Current model: %s\nRun /model <model> to persist a global model override, or /model default to restore the default.\nClaude Code Go does not provide the interactive model picker or model availability checks yet.",
				renderModelSetting(currentModel),
			),
		}, nil
	}

	if len(args.Raw) != 1 {
		return command.Result{}, fmt.Errorf("usage: %s", c.Metadata().Usage)
	}
	if c.Store == nil {
		return command.Result{}, fmt.Errorf("global model storage is not configured")
	}

	newModel := requested
	persistedValue := requested
	if requested == "default" {
		newModel = defaultModelSetting()
		persistedValue = ""
	}

	if err := c.Store.SaveModel(ctx, persistedValue); err != nil {
		return command.Result{}, err
	}
	if c.Config != nil {
		c.Config.Model = newModel
	}

	logger.DebugCF("commands", "updated model via model command", map[string]any{
		"previous_model": currentModel,
		"new_model":      newModel,
		"cleared":        persistedValue == "",
	})

	return command.Result{
		Output: fmt.Sprintf(
			"Model set to %s. Claude Code Go stores the preference now, but the interactive model picker and model availability checks are not implemented yet.",
			renderModelSetting(newModel),
		),
	}, nil
}

// currentModelValue returns the resolved current model or the stable runtime default when no config snapshot is available.
func currentModelValue(cfg *coreconfig.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.Model) == "" {
		return defaultModelSetting()
	}
	return cfg.Model
}

// defaultModelSetting returns the stable runtime default model label used when no explicit override is active.
func defaultModelSetting() string {
	return coreconfig.DefaultConfig().Model
}

// renderModelSetting appends one stable marker when the effective model matches the runtime default.
func renderModelSetting(model string) string {
	if model == defaultModelSetting() {
		return fmt.Sprintf("%s (default)", model)
	}
	return model
}
