package bootstrap

import (
	"fmt"
	"strings"

	platformconfig "github.com/sheepzhao/claude-code-go/internal/platform/config"
)

// EarlyCLIOptions stores bootstrap-time CLI options that must be handled before runtime config loads.
type EarlyCLIOptions struct {
	// SettingsValue stores one optional `--settings` override captured from CLI args.
	SettingsValue string
}

// ParseEarlyCLIOptions removes bootstrap-time flags from one argv slice and returns the remaining runtime args.
func ParseEarlyCLIOptions(args []string) (EarlyCLIOptions, []string, error) {
	var options EarlyCLIOptions
	filtered := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		current := strings.TrimSpace(args[index])
		switch {
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
		default:
			filtered = append(filtered, args[index])
		}
	}
	return options, filtered, nil
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

	app, err := NewAppWithDependencies(loader, DefaultEngineFactory)
	if err != nil {
		return nil, nil, err
	}
	return app, runArgs, nil
}
