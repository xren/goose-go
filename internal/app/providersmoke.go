package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
	"goose-go/internal/provider/openaicodex"
)

const defaultSmokePrompt = "Reply with the single word: pong"

type ProviderSmokeOptions struct {
	Debug bool
}

var newSmokeProvider = func(debugOut io.Writer) (provider.Provider, error) {
	if debugOut != nil {
		return openaicodex.New(openaicodex.WithDebugWriter(debugOut))
	}
	return openaicodex.New()
}

func RunProviderSmoke(ctx context.Context, out io.Writer, prompt string, opts ProviderSmokeOptions) error {
	if prompt == "" {
		prompt = defaultSmokePrompt
	}

	var debugOut io.Writer
	if opts.Debug {
		debugOut = out
	}

	p, err := newSmokeProvider(debugOut)
	if err != nil {
		return diagnoseProviderError("openai-codex", fmt.Errorf("create openai-codex provider: %w", err), opts.Debug)
	}

	stream, err := p.Stream(ctx, provider.Request{
		SystemPrompt: "You are a concise assistant.",
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text(prompt)),
		},
		Model: provider.ModelConfig{
			Provider: "openai-codex",
			Model:    "gpt-5-codex",
		},
	})
	if err != nil {
		return diagnoseProviderError("openai-codex", fmt.Errorf("start provider smoke request: %w", err), opts.Debug)
	}

	var sawDone bool
	var sawMessage bool

	for event := range stream {
		switch event.Type {
		case provider.EventTypeTextDelta:
			if _, err := io.WriteString(out, event.Delta); err != nil {
				return fmt.Errorf("write smoke output: %w", err)
			}
		case provider.EventTypeMessageComplete:
			sawMessage = true
		case provider.EventTypeDone:
			sawDone = true
		case provider.EventTypeError:
			return diagnoseProviderError("openai-codex", fmt.Errorf("provider smoke failed: %w", event.Err), opts.Debug)
		}
	}

	if _, err := io.WriteString(out, "\n"); err != nil {
		return fmt.Errorf("write smoke newline: %w", err)
	}

	if !sawMessage || !sawDone {
		return diagnoseProviderError("openai-codex", errors.New("provider smoke did not produce a complete response"), opts.Debug)
	}

	return nil
}

func ProviderSmokeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 90*time.Second)
}
