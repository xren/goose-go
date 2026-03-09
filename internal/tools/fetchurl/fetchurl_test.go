package fetchurl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"goose-go/internal/tools"
)

func TestFetchURLRunSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello from server"))
	}))
	defer server.Close()

	result, err := New().Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "fetch_url",
		Arguments: mustJSON(t, Arguments{URL: server.URL}),
	})
	if err != nil {
		t.Fatalf("run fetch_url: %v", err)
	}
	if result.IsError {
		t.Fatal("expected non-error result")
	}
	if got := result.Content[0].Text; got != "hello from server" {
		t.Fatalf("expected body text, got %q", got)
	}
}

func TestFetchURLRunCleansHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><h1>Hello</h1><p><strong>World</strong></p></body></html>`))
	}))
	defer server.Close()

	result, err := New().Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "fetch_url",
		Arguments: mustJSON(t, Arguments{URL: server.URL}),
	})
	if err != nil {
		t.Fatalf("run fetch_url: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Hello") || !strings.Contains(result.Content[0].Text, "World") {
		t.Fatalf("expected cleaned html text, got %q", result.Content[0].Text)
	}
}

func TestFetchURLRunTruncates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer server.Close()

	result, err := New().Run(context.Background(), tools.Call{
		ID:        "call_1",
		Name:      "fetch_url",
		Arguments: mustJSON(t, Arguments{URL: server.URL, MaxBytes: 4}),
	})
	if err != nil {
		t.Fatalf("run fetch_url: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "[truncated]") {
		t.Fatalf("expected truncated marker, got %q", result.Content[0].Text)
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}
