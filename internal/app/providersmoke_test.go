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
