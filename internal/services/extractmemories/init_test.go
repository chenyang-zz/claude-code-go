package extractmemories

import (
	"os"
	"testing"
)

func TestInitExtractMemories(t *testing.T) {
	envKey := "CLAUDE_FEATURE_" + FlagExtractMemories

	t.Run("creates system when enabled", func(t *testing.T) {
		os.Setenv(envKey, "1")
		defer os.Unsetenv(envKey)

		var registeredHook PostTurnHookFunc
		registerFn := func(hook PostTurnHookFunc) {
			registeredHook = hook
		}

		sys := InitExtractMemories(nil, registerFn, "/tmp/test-project")
		if sys == nil {
			t.Fatal("expected system to be created")
		}
		if registeredHook == nil {
			t.Error("expected hook to be registered")
		}

		sys.ResetForTesting()
	})

	t.Run("returns nil when disabled", func(t *testing.T) {
		os.Setenv(envKey, "0")
		defer os.Unsetenv(envKey)

		var registered bool
		registerFn := func(hook PostTurnHookFunc) {
			registered = true
		}

		sys := InitExtractMemories(nil, registerFn, "/tmp/test-project")
		if sys != nil {
			t.Error("expected nil system when disabled")
		}
		if registered {
			t.Error("expected hook NOT to be registered when disabled")
		}
	})

	t.Run("returns nil when auto memory disabled", func(t *testing.T) {
		os.Setenv(envKey, "1")
		os.Setenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY", "1")
		defer func() {
			os.Unsetenv(envKey)
			os.Unsetenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY")
		}()

		var registered bool
		registerFn := func(hook PostTurnHookFunc) {
			registered = true
		}

		sys := InitExtractMemories(nil, registerFn, "/tmp/test-project")
		if sys != nil {
			t.Error("expected nil system when auto memory disabled")
		}
		if registered {
			t.Error("expected hook NOT to be registered when auto memory disabled")
		}
	})

	t.Run("init with nil runner does not fail", func(t *testing.T) {
		os.Setenv(envKey, "1")
		defer os.Unsetenv(envKey)

		sys := InitExtractMemories(nil, func(hook PostTurnHookFunc) {}, "/tmp/test-project")
		if sys == nil {
			t.Fatal("expected system even with nil runner")
		}
		sys.ResetForTesting()
	})
}
