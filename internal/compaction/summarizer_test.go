package compaction

import (
	"context"
	"errors"
	"strings"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

func TestSummarizerUsesProviderAndReturnsSummary(t *testing.T) {
	stub := &summaryProviderStub{
		events: []provider.Event{
			{Type: provider.EventTypeUsage, Usage: &provider.Usage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30}},
			{
				Type: provider.EventTypeMessageComplete,
				Message: ptr(conversation.NewMessage(
					conversation.RoleAssistant,
					conversation.Text("## Goal\nShip compaction"),
				)),
			},
			{Type: provider.EventTypeDone},
		},
	}

	summarizer, err := NewSummarizer(stub, provider.ModelConfig{
		Provider: "openai-codex",
		Model:    "gpt-5-codex",
	})
	if err != nil {
		t.Fatalf("new summarizer: %v", err)
	}

	result, err := summarizer.Summarize(context.Background(), "sess_1", SummaryRequest{
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("please continue this work")),
		},
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}

	if result.Summary != "## Goal\nShip compaction" {
		t.Fatalf("unexpected summary %q", result.Summary)
	}
	if result.Usage.TotalTokens != 30 {
		t.Fatalf("expected usage to be preserved, got %+v", result.Usage)
	}
	if stub.request == nil {
		t.Fatal("expected provider request")
	}
	if stub.request.SystemPrompt == "" {
		t.Fatal("expected embedded system prompt")
	}
	if got := len(stub.request.Messages); got != 1 {
		t.Fatalf("expected one summarization message, got %d", got)
	}
}

func TestSummarizerIncludesPreviousSummaryAndCustomInstructions(t *testing.T) {
	stub := &summaryProviderStub{
		events: []provider.Event{
			{
				Type: provider.EventTypeMessageComplete,
				Message: ptr(conversation.NewMessage(
					conversation.RoleAssistant,
					conversation.Text("updated summary"),
				)),
			},
		},
	}

	summarizer, err := NewSummarizer(stub, provider.ModelConfig{
		Provider: "openai-codex",
		Model:    "gpt-5-codex",
	})
	if err != nil {
		t.Fatalf("new summarizer: %v", err)
	}

	_, err = summarizer.Summarize(context.Background(), "sess_1", SummaryRequest{
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("new work")),
		},
		PreviousSummary:    "old summary",
		CustomInstructions: "focus on errors",
	})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}

	got := stub.request.Messages[0].Content[0].Text.Text
	if !strings.Contains(got, "<previous-summary>\nold summary\n</previous-summary>") {
		t.Fatalf("expected previous summary in request, got %q", got)
	}
	if !strings.Contains(got, "Additional focus:\nfocus on errors") {
		t.Fatalf("expected custom instructions in request, got %q", got)
	}
}

func TestSummarizerPropagatesProviderError(t *testing.T) {
	stub := &summaryProviderStub{
		events: []provider.Event{
			{Type: provider.EventTypeError, Err: errors.New("boom")},
		},
	}

	summarizer, err := NewSummarizer(stub, provider.ModelConfig{
		Provider: "openai-codex",
		Model:    "gpt-5-codex",
	})
	if err != nil {
		t.Fatalf("new summarizer: %v", err)
	}

	_, err = summarizer.Summarize(context.Background(), "sess_1", SummaryRequest{
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("new work")),
		},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
}

func TestSummarizerRejectsEmptySummary(t *testing.T) {
	stub := &summaryProviderStub{
		events: []provider.Event{
			{
				Type: provider.EventTypeMessageComplete,
				Message: ptr(conversation.NewMessage(
					conversation.RoleAssistant,
					conversation.SystemNotification("info", "no text", nil),
				)),
			},
		},
	}

	summarizer, err := NewSummarizer(stub, provider.ModelConfig{
		Provider: "openai-codex",
		Model:    "gpt-5-codex",
	})
	if err != nil {
		t.Fatalf("new summarizer: %v", err)
	}

	_, err = summarizer.Summarize(context.Background(), "sess_1", SummaryRequest{
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("new work")),
		},
	})
	if err == nil {
		t.Fatal("expected empty summary error")
	}
}

type summaryProviderStub struct {
	request *provider.Request
	events  []provider.Event
}

func (s *summaryProviderStub) Stream(_ context.Context, req provider.Request) (<-chan provider.Event, error) {
	s.request = &req
	ch := make(chan provider.Event, len(s.events))
	for _, event := range s.events {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func ptr[T any](v T) *T { return &v }
