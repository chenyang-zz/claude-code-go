package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	coreconfig "github.com/sheepzhao/claude-code-go/internal/core/config"
	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
)

// EarlyCLIOptions stores bootstrap-time CLI options that must be handled before runtime config loads.
type EarlyCLIOptions struct {
	// SettingsValue stores one optional `--settings` override captured from CLI args.
	SettingsValue string
	// SettingSources restricts which disk-backed settings files should be loaded when `--setting-sources` is present.
	SettingSources []platformconfig.SettingSource
	// HasSettingSources reports whether `--setting-sources` was explicitly provided, including the empty-string case.
	HasSettingSources bool
	// RemoteEnabled reports whether `--remote` was explicitly provided.
	RemoteEnabled bool
	// RemoteDescription stores the optional `--remote` description consumed during bootstrap parsing.
	RemoteDescription string
}

// ParseEarlyCLIOptions removes bootstrap-time flags from one argv slice and returns the remaining runtime args.
func ParseEarlyCLIOptions(args []string) (EarlyCLIOptions, []string, error) {
	var options EarlyCLIOptions
	filtered := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		current := strings.TrimSpace(args[index])
		switch {
		case current == "--remote":
			options.RemoteEnabled = true
			if index+1 < len(args) && shouldConsumeRemoteDescription(args[index+1]) {
				index++
				if options.RemoteDescription == "" {
					options.RemoteDescription = args[index]
				}
			}
		case strings.HasPrefix(current, "--remote="):
			options.RemoteEnabled = true
			if options.RemoteDescription == "" {
				options.RemoteDescription = strings.TrimPrefix(current, "--remote=")
			}
		case current == "--settings":
			if index+1 >= len(args) {
				return EarlyCLIOptions{}, nil, fmt.Errorf("missing value for --settings")
			}
			index++
			if options.SettingsValue == "" {
				options.SettingsValue = args[index]
			}
		case strings.HasPrefix(current, "--settings="):
			if options.SettingsValue == "" {
				options.SettingsValue = strings.TrimPrefix(current, "--settings=")
			}
		case current == "--setting-sources":
			if index+1 >= len(args) {
				return EarlyCLIOptions{}, nil, fmt.Errorf("missing value for --setting-sources")
			}
			index++
			if !options.HasSettingSources {
				sources, err := platformconfig.ParseSettingSourcesFlag(args[index])
				if err != nil {
					return EarlyCLIOptions{}, nil, err
				}
				options.SettingSources = sources
				options.HasSettingSources = true
			}
		case strings.HasPrefix(current, "--setting-sources="):
			if !options.HasSettingSources {
				sources, err := platformconfig.ParseSettingSourcesFlag(strings.TrimPrefix(current, "--setting-sources="))
				if err != nil {
					return EarlyCLIOptions{}, nil, err
				}
				options.SettingSources = sources
				options.HasSettingSources = true
			}
		default:
			filtered = append(filtered, args[index])
		}
	}
	return options, filtered, nil
}

// shouldConsumeRemoteDescription reports whether one CLI token should be consumed as the optional `--remote` description.
func shouldConsumeRemoteDescription(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "/") {
		return false
	}
	return true
}

// NewAppFromArgs builds the production app after consuming the bootstrap-time CLI flags that affect config loading.
func NewAppFromArgs(args []string) (*App, []string, error) {
	options, runArgs, err := ParseEarlyCLIOptions(args)
	if err != nil {
		return nil, nil, err
	}

	loader, err := platformconfig.NewDefaultFileLoader()
	if err != nil {
		return nil, nil, err
	}
	loader.FlagSettingsValue = options.SettingsValue
	if options.HasSettingSources {
		loader.AllowedSettingSources = options.SettingSources
	}

	app, err := NewAppWithDependencies(earlyOptionsLoader{
		base:    loader,
		options: options,
	}, DefaultEngineFactory)
	if err != nil {
		return nil, nil, err
	}
	return app, runArgs, nil
}

// earlyOptionsLoader applies bootstrap-time CLI choices onto the resolved runtime config before the app is wired.
type earlyOptionsLoader struct {
	base    coreconfig.Loader
	options EarlyCLIOptions
}

// Load resolves the underlying config and injects any bootstrap-time remote session context.
func (l earlyOptionsLoader) Load(ctx context.Context) (coreconfig.Config, error) {
	cfg, err := l.base.Load(ctx)
	if err != nil {
		return coreconfig.Config{}, err
	}
	if !l.options.RemoteEnabled {
		return cfg, nil
	}

	sessionID := fmt.Sprintf("session_%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
	cfg.RemoteSession = coreconfig.RemoteSessionConfig{
		Enabled:       true,
		SessionID:     sessionID,
		URL:           coreconfig.BuildRemoteSessionURL(sessionID),
		InitialPrompt: l.options.RemoteDescription,
	}
	return cfg, nil
}
