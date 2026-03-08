package compaction

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

//go:embed compaction_prompt.md
var compactionPrompt string

type SummaryRequest struct {
	Messages           []conversation.Message
	PreviousSummary    string
	CustomInstructions string
}

type SummaryResult struct {
	Summary string         `json:"summary"`
	Usage   provider.Usage `json:"usage"`
}

type Summarizer struct {
	provider provider.Provider
	model    provider.ModelConfig
}

func NewSummarizer(p provider.Provider, model provider.ModelConfig) (*Summarizer, error) {
	if p == nil {
		return nil, errors.New("provider is required")
	}
	if err := model.Validate(); err != nil {
		return nil, fmt.Errorf("model: %w", err)
	}
	return &Summarizer{provider: p, model: model}, nil
}

func (s *Summarizer) Summarize(ctx context.Context, sessionID string, req SummaryRequest) (SummaryResult, error) {
	if len(req.Messages) == 0 {
		return SummaryResult{}, errors.New("messages are required")
	}

	stream, err := s.provider.Stream(ctx, provider.Request{
		SessionID:    sessionID,
		SystemPrompt: compactionPrompt,
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text(buildSummaryRequestText(req))),
		},
		Tools: nil,
		Model: provider.ModelConfig{
			Provider:        s.model.Provider,
			Model:           s.model.Model,
			Temperature:     s.model.Temperature,
			MaxOutputTokens: nonZero(s.model.MaxOutputTokens, 2048),
		},
	})
	if err != nil {
		return SummaryResult{}, fmt.Errorf("start compaction summarization: %w", err)
	}

	var (
		finalMessage *conversation.Message
		lastUsage    *provider.Usage
	)
	for event := range stream {
		switch event.Type {
		case provider.EventTypeUsage:
			lastUsage = event.Usage
		case provider.EventTypeMessageComplete:
			finalMessage = event.Message
		case provider.EventTypeError:
			return SummaryResult{}, fmt.Errorf("compaction summarization failed: %w", event.Err)
		}
	}

	if finalMessage == nil {
		return SummaryResult{}, errors.New("compaction summarization produced no final message")
	}

	summary := extractSummaryText(*finalMessage)
	if summary == "" {
		return SummaryResult{}, errors.New("compaction summarization produced empty text")
	}

	var usage provider.Usage
	if lastUsage != nil {
		usage = *lastUsage
	}

	return SummaryResult{
		Summary: summary,
		Usage:   usage,
	}, nil
}

func buildSummaryRequestText(req SummaryRequest) string {
	var builder strings.Builder
	builder.WriteString("<conversation>\n")
	builder.WriteString(SerializeForSummarization(req.Messages))
	builder.WriteString("\n</conversation>")

	if strings.TrimSpace(req.PreviousSummary) != "" {
		builder.WriteString("\n\n<previous-summary>\n")
		builder.WriteString(strings.TrimSpace(req.PreviousSummary))
		builder.WriteString("\n</previous-summary>")
		builder.WriteString("\n\nUpdate the summary to include the new conversation details while preserving still-relevant prior context.")
	}

	if strings.TrimSpace(req.CustomInstructions) != "" {
		builder.WriteString("\n\nAdditional focus:\n")
		builder.WriteString(strings.TrimSpace(req.CustomInstructions))
	}

	return builder.String()
}

func extractSummaryText(message conversation.Message) string {
	parts := make([]string, 0, len(message.Content))
	for _, item := range message.Content {
		if item.Type == conversation.ContentTypeText && item.Text != nil && item.Text.Text != "" {
			parts = append(parts, item.Text.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func nonZero(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
