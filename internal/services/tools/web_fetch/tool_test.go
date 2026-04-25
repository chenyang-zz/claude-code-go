package web_fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	corepermission "github.com/sheepzhao/claude-code-go/internal/core/permission"
	coretool "github.com/sheepzhao/claude-code-go/internal/core/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTool_Name(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	assert.Equal(t, "WebFetch", tool.Name())
}

func TestTool_IsReadOnly(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	assert.True(t, tool.IsReadOnly())
}

func TestTool_IsConcurrencySafe(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	assert.True(t, tool.IsConcurrencySafe())
}

func TestTool_InputSchema(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	schema := tool.InputSchema()
	assert.Contains(t, schema.Properties, "url")
	assert.Contains(t, schema.Properties, "prompt")
	assert.True(t, schema.Properties["url"].Required)
}

func TestTool_Invoke_MissingURL(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Error, "missing required field \"url\"")
}

func TestTool_Invoke_InvalidURL(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": "https://localhost"},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Error, "Invalid URL")
}

func TestTool_Invoke_PreapprovedAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body><p>Hello</p></body></html>")
	}))
	defer server.Close()

	// Allow all domains for this test
	tool := NewTool(nil, []string{"*"}, nil, nil)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": server.URL, "prompt": "summarize"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Output, "Hello")
}

func TestTool_Invoke_Denied(t *testing.T) {
	tool := NewTool(nil, nil, []string{"example.com"}, nil)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": "https://example.com/page"},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Error, "denied")
}

func TestTool_Invoke_AskRequiresApproval(t *testing.T) {
	tool := NewTool(nil, nil, nil, nil)
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": "https://unknown-site.com/page"},
	})
	require.Error(t, err)
	var permErr *corepermission.WebFetchPermissionError
	require.True(t, assert.ErrorAs(t, err, &permErr))
	assert.Equal(t, corepermission.DecisionAsk, permErr.Decision)
}

func TestTool_Invoke_Granted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body><p>Granted content</p></body></html>")
	}))
	defer server.Close()

	// Allow all domains for this test
	tool := NewTool(nil, []string{"*"}, nil, nil)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": server.URL, "prompt": "test"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Output, "Granted content")
}

func TestTool_Invoke_Redirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "https://other.example.com/dest")
			w.WriteHeader(http.StatusMovedPermanently)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Allow all domains for this test
	tool := NewTool(nil, []string{"*"}, nil, nil)
	result, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": server.URL + "/redirect", "prompt": "test"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.Error)
	assert.Contains(t, result.Output, "REDIRECT DETECTED")
	assert.Contains(t, result.Output, "https://other.example.com/dest")
}

func TestTool_Invoke_NilReceiver(t *testing.T) {
	var tool *Tool
	_, err := tool.Invoke(context.Background(), coretool.Call{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil receiver")
}

func TestTool_Invoke_NilFetcher(t *testing.T) {
	tool := &Tool{permissions: NewPermissionChecker(nil, nil, nil)}
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": "https://example.com"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetcher is not configured")
}

func TestTool_Invoke_NilPermissions(t *testing.T) {
	tool := &Tool{fetcher: NewFetcher(nil)}
	_, err := tool.Invoke(context.Background(), coretool.Call{
		Input: map[string]any{"url": "https://example.com"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "permission checker is not configured")
}
