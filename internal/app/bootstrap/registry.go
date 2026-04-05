package bootstrap

import (
	"github.com/sheepzhao/claude-code-go/internal/core/command"
)

// RegistrySet summarizes the minimum production registries wired into the app.
type RegistrySet struct {
	// CommandRegistry stores slash commands exposed to the REPL.
	CommandRegistry command.Registry
	// Commands counts registered slash commands for observability and tests.
	Commands int
	// Tools counts registered tools in the runtime tool catalog.
	Tools int
}
