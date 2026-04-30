package plugin

import (
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/platform/lsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLspRegistrar_RegisterLspServers_Success(t *testing.T) {
	manager := lsp.NewManager()
	registrar := NewLspRegistrar(manager)

	servers := []*LspServerConfig{
		{
			Name:                "go-server",
			Command:             "/usr/bin/gopls",
			Args:                []string{"serve"},
			Env:                 map[string]string{"GO111MODULE": "on"},
			ExtensionToLanguage: map[string]string{".go": "go", ".mod": "go"},
		},
	}

	registered, errs := registrar.RegisterLspServers(servers)
	require.Len(t, errs, 0)
	assert.Equal(t, 1, registered)
}

func TestLspRegistrar_RegisterLspServers_NilManager(t *testing.T) {
	registrar := NewLspRegistrar(nil)

	servers := []*LspServerConfig{
		{Name: "go-server", Command: "/usr/bin/gopls", ExtensionToLanguage: map[string]string{".go": "go"}},
	}

	registered, errs := registrar.RegisterLspServers(servers)
	require.Len(t, errs, 0)
	assert.Equal(t, 0, registered)
}

func TestLspRegistrar_RegisterLspServers_Duplicate(t *testing.T) {
	manager := lsp.NewManager()
	registrar := NewLspRegistrar(manager)

	servers := []*LspServerConfig{
		{Name: "dup-server", Command: "/bin/a", ExtensionToLanguage: map[string]string{".a": "a"}},
		{Name: "dup-server", Command: "/bin/b", ExtensionToLanguage: map[string]string{".b": "b"}},
	}

	registered, errs := registrar.RegisterLspServers(servers)
	assert.Equal(t, 1, registered)
	require.Len(t, errs, 1)
	assert.Equal(t, "lsp-registration-error", errs[0].Type)
}

func TestLspRegistrar_RegisterLspServers_SkipsNilEntries(t *testing.T) {
	manager := lsp.NewManager()
	registrar := NewLspRegistrar(manager)

	servers := []*LspServerConfig{
		{Name: "valid", Command: "/bin/valid", ExtensionToLanguage: map[string]string{".v": "v"}},
		nil,
	}

	registered, errs := registrar.RegisterLspServers(servers)
	require.Len(t, errs, 0)
	assert.Equal(t, 1, registered)
}

func TestLspRegistrar_RegisterLspServers_Empty(t *testing.T) {
	manager := lsp.NewManager()
	registrar := NewLspRegistrar(manager)

	registered, errs := registrar.RegisterLspServers([]*LspServerConfig{})
	require.Len(t, errs, 0)
	assert.Equal(t, 0, registered)
}

func TestToLspServerConfig(t *testing.T) {
	cfg := &LspServerConfig{
		Command: "/usr/bin/gopls",
		Args:    []string{"serve", "-rpc.trace"},
		Env:     map[string]string{"KEY": "val"},
	}

	core := toLspServerConfig(cfg)
	assert.Equal(t, "/usr/bin/gopls", core.Command)
	assert.Equal(t, []string{"serve", "-rpc.trace"}, core.Args)
	assert.Equal(t, map[string]string{"KEY": "val"}, core.Env)
}

func TestExtractExtensions(t *testing.T) {
	assert.Nil(t, extractExtensions(nil))
	assert.Nil(t, extractExtensions(map[string]string{}))

	exts := extractExtensions(map[string]string{".go": "go", ".mod": "go", ".ts": "typescript"})
	require.Len(t, exts, 3)
	assert.Contains(t, exts, ".go")
	assert.Contains(t, exts, ".mod")
	assert.Contains(t, exts, ".ts")
}
