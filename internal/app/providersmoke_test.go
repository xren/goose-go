package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

func TestRunProviderSmokeConsumesStream(t *testing.T) {
	originalFactory := newSmokeProvider
	t.Cleanup(func() {
		newSmokeProvider = originalFactory
	})

	newSmokeProvider = func(io.Writer) (provider.Provider, error) {
		return smokeProviderStub{
			events: []provider.Event{
				{Type: provider.EventTypeTextDelta, Delta: "pong"},
				{
					Type: provider.EventTypeMessageComplete,
					Message: ptr(conversation.NewMessage(
						conversation.RoleAssistant,
						conversation.Text("pong"),
					)),
				},
				{Type: provider.EventTypeDone},
			},
		}, nil
	}

	var out bytes.Buffer
	if err := RunProviderSmoke(context.Background(), &out, "ping", ProviderSmokeOptions{}); err != nil {
		t.Fatalf("run provider smoke: %v", err)
	}

	if got := out.String(); got != "pong\n" {
		t.Fatalf("expected pong output, got %q", got)
	}
}

func TestRunProviderSmokeReturnsProviderError(t *testing.T) {
	originalFactory := newSmokeProvider
	t.Cleanup(func() {
		newSmokeProvider = originalFactory
	})

	newSmokeProvider = func(io.Writer) (provider.Provider, error) {
		return smokeProviderStub{
			events: []provider.Event{
				{Type: provider.EventTypeError, Err: errors.New("boom")},
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunProviderSmoke(context.Background(), &out, "ping", ProviderSmokeOptions{})
	if err == nil {
		t.Fatal("expected smoke error")
	}
	var diag *DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("expected diagnostic error, got %T", err)
	}
	if diag.Category != DiagnosticUnknown {
		t.Fatalf("expected unknown diagnostic category, got %q", diag.Category)
	}
}

func TestRunProviderSmokeClassifiesAuthMissing(t *testing.T) {
	originalFactory := newSmokeProvider
	t.Cleanup(func() {
		newSmokeProvider = originalFactory
	})

	newSmokeProvider = func(io.Writer) (provider.Provider, error) {
		return nil, errors.New("codex auth file not found at /tmp/auth.json; run `codex login`")
	}

	var out bytes.Buffer
	err := RunProviderSmoke(context.Background(), &out, "ping", ProviderSmokeOptions{})
	var diag *DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("expected diagnostic error, got %T", err)
	}
	if diag.Category != DiagnosticAuthMissing {
		t.Fatalf("expected auth_missing, got %q", diag.Category)
	}
}

func TestRunProviderSmokeClassifiesHTTPError(t *testing.T) {
	originalFactory := newSmokeProvider
	t.Cleanup(func() {
		newSmokeProvider = originalFactory
	})

	newSmokeProvider = func(io.Writer) (provider.Provider, error) {
		return smokeProviderStub{
			events: []provider.Event{
				{Type: provider.EventTypeError, Err: errors.New("codex request failed: status 401: nope")},
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunProviderSmoke(context.Background(), &out, "ping", ProviderSmokeOptions{})
	var diag *DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("expected diagnostic error, got %T", err)
	}
	if diag.Category != DiagnosticProviderHTTP {
		t.Fatalf("expected provider_http_error, got %q", diag.Category)
	}
}

func TestRunProviderSmokeClassifiesIncompleteResponse(t *testing.T) {
	originalFactory := newSmokeProvider
	t.Cleanup(func() {
		newSmokeProvider = originalFactory
	})

	newSmokeProvider = func(io.Writer) (provider.Provider, error) {
		return smokeProviderStub{
			events: []provider.Event{
				{Type: provider.EventTypeTextDelta, Delta: "partial"},
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunProviderSmoke(context.Background(), &out, "ping", ProviderSmokeOptions{})
	var diag *DiagnosticError
	if !errors.As(err, &diag) {
		t.Fatalf("expected diagnostic error, got %T", err)
	}
	if diag.Category != DiagnosticProviderEmpty {
		t.Fatalf("expected provider_empty_response, got %q", diag.Category)
	}
}

func TestRunProviderSmokeDebugIncludesCause(t *testing.T) {
	originalFactory := newSmokeProvider
	t.Cleanup(func() {
		newSmokeProvider = originalFactory
	})

	newSmokeProvider = func(io.Writer) (provider.Provider, error) {
		return smokeProviderStub{
			events: []provider.Event{
				{Type: provider.EventTypeError, Err: errors.New("send codex request: dial tcp timeout")},
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunProviderSmoke(context.Background(), &out, "ping", ProviderSmokeOptions{Debug: true})
	if err == nil {
		t.Fatal("expected smoke error")
	}
	if got := err.Error(); got == "" || !containsText(got, "request could not be sent", "dial tcp timeout") {
		t.Fatalf("expected debug error to include summary and cause, got %q", got)
	}
}

type smokeProviderStub struct {
	events []provider.Event
}

func (s smokeProviderStub) Stream(context.Context, provider.Request) (<-chan provider.Event, error) {
	ch := make(chan provider.Event, len(s.events))
	for _, event := range s.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func ptr[T any](value T) *T {
	return &value
}

func containsText(text string, parts ...string) bool {
	for _, part := range parts {
		if !bytes.Contains([]byte(text), []byte(part)) {
			return false
		}
	}
	return true
}
