package models

import (
	"context"
	"errors"
	"testing"

	"goose-go/internal/auth/codex"
)

type fakeCodexLoader struct{ err error }

func (f fakeCodexLoader) Resolve(context.Context) (codex.Credentials, error) {
	if f.err != nil {
		return codex.Credentials{}, f.err
	}
	return codex.Credentials{AccountID: "acct_1", AccessToken: "tok"}, nil
}

func TestResolveSelectionDefaultModel(t *testing.T) {
	provider, model, err := ResolveSelection(string(ProviderOpenAICodex), "")
	if err != nil {
		t.Fatalf("resolve selection: %v", err)
	}
	if provider.ID != ProviderOpenAICodex {
		t.Fatalf("unexpected provider: %#v", provider)
	}
	if model.ID != ModelGPT5Codex {
		t.Fatalf("expected default model %q, got %q", ModelGPT5Codex, model.ID)
	}
}

func TestResolveSelectionRejectsUnknownModel(t *testing.T) {
	_, _, err := ResolveSelection(string(ProviderOpenAICodex), "unknown")
	if err == nil {
		t.Fatal("expected error for unknown model")
	}
}

func TestAvailabilityResolverMarksUnavailableWhenAuthFails(t *testing.T) {
	resolver := NewResolverWithCodex(fakeCodexLoader{err: errors.New("missing codex auth")})
	available, err := resolver.ListAvailable(context.Background(), string(ProviderOpenAICodex))
	if err != nil {
		t.Fatalf("list available: %v", err)
	}
	if len(available) == 0 {
		t.Fatal("expected models")
	}
	for _, item := range available {
		if item.Available {
			t.Fatalf("expected unavailable model, got %#v", item)
		}
		if item.Reason == "" {
			t.Fatalf("expected unavailable reason, got %#v", item)
		}
	}
}

func TestAvailabilityResolverMarksAvailableWhenAuthSucceeds(t *testing.T) {
	resolver := NewResolverWithCodex(fakeCodexLoader{})
	available, err := resolver.ListAvailable(context.Background(), string(ProviderOpenAICodex))
	if err != nil {
		t.Fatalf("list available: %v", err)
	}
	for _, item := range available {
		if !item.Available {
			t.Fatalf("expected available model, got %#v", item)
		}
	}
}
