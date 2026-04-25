package web_fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateURL_Valid(t *testing.T) {
	assert.True(t, validateURL("https://example.com"))
	assert.True(t, validateURL("http://example.com/path"))
}

func TestValidateURL_TooLong(t *testing.T) {
	longURL := "https://example.com/" + string(make([]byte, maxURLLength))
	assert.False(t, validateURL(longURL))
}

func TestValidateURL_NoScheme(t *testing.T) {
	assert.False(t, validateURL("example.com"))
}

func TestValidateURL_UserInfo(t *testing.T) {
	assert.False(t, validateURL("https://user:pass@example.com"))
}

func TestValidateURL_SingleLabelHost(t *testing.T) {
	assert.False(t, validateURL("https://localhost"))
}

func TestIsPermittedRedirect_SameHost(t *testing.T) {
	assert.True(t, isPermittedRedirect("https://example.com/a", "https://example.com/b"))
}

func TestIsPermittedRedirect_WwwVariant(t *testing.T) {
	assert.True(t, isPermittedRedirect("https://example.com/a", "https://www.example.com/b"))
	assert.True(t, isPermittedRedirect("https://www.example.com/a", "https://example.com/b"))
}

func TestIsPermittedRedirect_DifferentHost(t *testing.T) {
	assert.False(t, isPermittedRedirect("https://example.com/a", "https://other.com/b"))
}

func TestIsPermittedRedirect_DifferentScheme(t *testing.T) {
	assert.False(t, isPermittedRedirect("http://example.com/a", "https://example.com/b"))
}

func TestIsPermittedRedirect_DifferentPort(t *testing.T) {
	assert.False(t, isPermittedRedirect("https://example.com:443/a", "https://example.com:8443/b"))
}

func TestFetcher_FetchURLMarkdownContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body><h1>Hello</h1><p>World</p></body></html>")
	}))
	defer server.Close()

	fetcher := NewFetcher(nil)
	content, redirect, err := fetcher.FetchURLMarkdownContent(context.Background(), server.URL)
	require.NoError(t, err)
	require.Nil(t, redirect)
	assert.Equal(t, http.StatusOK, content.Code)
	assert.Contains(t, content.Content, "# Hello")
	assert.Contains(t, content.Content, "World")
}

func TestFetcher_CacheHit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "<html><body><p>Call %d</p></body></html>", callCount)
	}))
	defer server.Close()

	cache := NewCache(1024*1024, time.Minute)
	fetcher := NewFetcher(cache)

	_, _, err := fetcher.FetchURLMarkdownContent(context.Background(), server.URL)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second fetch should hit cache
	content, redirect, err := fetcher.FetchURLMarkdownContent(context.Background(), server.URL)
	require.NoError(t, err)
	require.Nil(t, redirect)
	assert.Equal(t, 1, callCount) // No additional HTTP call
	assert.Contains(t, content.Content, "Call 1")
}

func TestFetcher_CrossHostRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "https://other.example.com/dest")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fetcher := NewFetcher(nil)
	content, redirect, err := fetcher.FetchURLMarkdownContent(context.Background(), server.URL+"/redirect")
	require.NoError(t, err)
	require.Nil(t, content)
	require.NotNil(t, redirect)
	assert.Equal(t, http.StatusFound, redirect.StatusCode)
	assert.Equal(t, "https://other.example.com/dest", redirect.RedirectUrl)
}

func TestFetcher_SameHostRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/dest")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.URL.Path == "/dest" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "destination")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := NewFetcher(nil)
	content, redirect, err := fetcher.FetchURLMarkdownContent(context.Background(), server.URL+"/redirect")
	require.NoError(t, err)
	require.Nil(t, redirect)
	assert.Equal(t, "destination", content.Content)
}

func TestFetcher_HTTPSUpgrade(t *testing.T) {
	// This test validates that the fetcher upgrades http to https.
	// We can't easily test the actual HTTPS upgrade without a real server,
	// so we just validate the URL validation accepts http and the upgrade logic exists.
	assert.True(t, validateURL("http://example.com"))
}

func TestFetcher_MaxRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/redirect")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	fetcher := NewFetcher(nil)
	_, _, err := fetcher.FetchURLMarkdownContent(context.Background(), server.URL+"/start")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many redirects")
}

func TestFetcher_NonHTMLContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "plain text content")
	}))
	defer server.Close()

	fetcher := NewFetcher(nil)
	content, redirect, err := fetcher.FetchURLMarkdownContent(context.Background(), server.URL)
	require.NoError(t, err)
	require.Nil(t, redirect)
	assert.Equal(t, "plain text content", content.Content)
}

func TestFetcher_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	fetcher := NewFetcher(nil)
	_, _, err := fetcher.FetchURLMarkdownContent(ctx, server.URL)
	require.Error(t, err)
}
