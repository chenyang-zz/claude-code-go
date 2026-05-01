package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/sheepzhao/claude-code-go/internal/core/model"
)

func TestTryFallback_FallbackModelSucceeds(t *testing.T) {
	fbModelClient := &fakeModelClient{streamFn: func(_ context.Context, req model.Request) (model.Stream, error) {
		if req.Model != "fallback-model" {
			t.Fatalf("fallback model = %q, want fallback-model", req.Model)
		}
		return make(chan model.Event), nil
	}}

	e := &Runtime{
		Client:        fbModelClient,
		FallbackModel: "fallback-model",
	}

	fb := e.tryFallback(context.Background(), model.Request{Model: "primary-model"}, errors.New("timeout"))
	if fb == nil {
		t.Fatal("tryFallback() returned nil, want fallback result")
	}
	if fb.model != "fallback-model" {
		t.Fatalf("fallback model = %q, want fallback-model", fb.model)
	}
}

func TestTryFallback_FallbackModelFailsThenClients(t *testing.T) {
	fbModelClient := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return nil, errors.New("fallback model also fails")
	}}
	clientA := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return nil, errors.New("client A fails")
	}}
	clientB := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return make(chan model.Event), nil
	}}

	e := &Runtime{
		Client:          fbModelClient, // FallbackModel uses the same Client
		FallbackModel:   "fallback-model",
		FallbackClients: []model.Client{clientA, clientB},
	}

	fb := e.tryFallback(context.Background(), model.Request{Model: "primary"}, errors.New("timeout"))
	if fb == nil {
		t.Fatal("tryFallback() returned nil, want fallback result from client B")
	}
	if fb.model != "fallback-client-1" {
		t.Fatalf("fallback model = %q, want fallback-client-1", fb.model)
	}

	if len(clientA.requests) != 1 {
		t.Fatalf("client A requests = %d, want 1", len(clientA.requests))
	}
	if len(clientB.requests) != 1 {
		t.Fatalf("client B requests = %d, want 1", len(clientB.requests))
	}
}

func TestTryFallback_NoFallbackConfigured(t *testing.T) {
	e := &Runtime{Client: &fakeModelClient{}}
	fb := e.tryFallback(context.Background(), model.Request{}, errors.New("timeout"))
	if fb != nil {
		t.Fatal("tryFallback() should return nil when no fallback is configured")
	}
}

func TestTryFallback_NonRetryableError(t *testing.T) {
	e := &Runtime{
		Client:        &fakeModelClient{},
		FallbackModel: "fb",
	}
	fb := e.tryFallback(context.Background(), model.Request{}, errors.New("permanent error"))
	if fb != nil {
		t.Fatal("tryFallback() should return nil for non-retryable errors")
	}
}

func TestTryFallback_ClientsOnlyNoFallbackModel(t *testing.T) {
	clientA := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return make(chan model.Event), nil
	}}

	e := &Runtime{
		Client:          &fakeModelClient{},
		FallbackClients: []model.Client{clientA},
	}

	fb := e.tryFallback(context.Background(), model.Request{}, errors.New("timeout"))
	if fb == nil {
		t.Fatal("tryFallback() returned nil, want fallback result from client A")
	}
	if fb.model != "fallback-client-0" {
		t.Fatalf("fallback model = %q, want fallback-client-0", fb.model)
	}
}

func TestTryFallback_SkipsNilClients(t *testing.T) {
	clientB := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return make(chan model.Event), nil
	}}

	e := &Runtime{
		Client:          &fakeModelClient{},
		FallbackClients: []model.Client{nil, clientB},
	}

	fb := e.tryFallback(context.Background(), model.Request{}, errors.New("timeout"))
	if fb == nil {
		t.Fatal("tryFallback() returned nil, want fallback result from client B")
	}
	if fb.model != "fallback-client-1" {
		t.Fatalf("fallback model = %q, want fallback-client-1", fb.model)
	}
}

func TestTryFallback_AllClientsFail(t *testing.T) {
	clientA := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return nil, errors.New("fail A")
	}}
	clientB := &fakeModelClient{streamFn: func(_ context.Context, _ model.Request) (model.Stream, error) {
		return nil, errors.New("fail B")
	}}

	e := &Runtime{
		Client:          &fakeModelClient{},
		FallbackClients: []model.Client{clientA, clientB},
	}

	fb := e.tryFallback(context.Background(), model.Request{}, errors.New("timeout"))
	if fb != nil {
		t.Fatal("tryFallback() should return nil when all clients fail")
	}
}
