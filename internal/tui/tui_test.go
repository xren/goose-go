package tui

import (
	"context"
	"errors"
	"testing"
	"time"

	"goose-go/internal/agent"
	"goose-go/internal/app"
	"goose-go/internal/conversation"
	"goose-go/internal/session"
)

type fakeRuntime struct{}

type fakeTraceWriter struct{}

func (fakeRuntime) LoadOrCreateSession(context.Context, string, string) (session.Session, int, error) {
	return session.Session{}, 0, errors.New("not implemented")
}

func (fakeRuntime) ReplayConversation(context.Context, string) (session.Session, error) {
	return session.Session{}, errors.New("not implemented")
}

func (fakeRuntime) OpenTraceWriter(string) (app.EventRecorder, error) {
	return fakeTraceWriter{}, nil
}

func (fakeRuntime) ReplyStream(context.Context, string, string) (<-chan agent.Event, error) {
	ch := make(chan agent.Event)
	close(ch)
	return ch, nil
}

func (fakeRuntime) WorkingDir() string { return "/tmp/work" }

func (fakeTraceWriter) Write(agent.Event) error { return nil }
func (fakeTraceWriter) Close() error            { return nil }

func TestBuildTranscriptFromConversation(t *testing.T) {
	msgTime := time.Now().UTC()
	conv := conversation.Conversation{Messages: []conversation.Message{
		{ID: "m1", Role: conversation.RoleUser, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("hello")}},
		{ID: "m2", Role: conversation.RoleAssistant, CreatedAt: msgTime, Content: []conversation.Content{conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`))}},
		{ID: "m3", Role: conversation.RoleTool, CreatedAt: msgTime, Content: []conversation.Content{conversation.ToolResponse("call_1", false, []conversation.ToolResult{{Type: "text", Text: "/tmp/work"}}, nil)}},
		{ID: "m4", Role: conversation.RoleAssistant, CreatedAt: msgTime, Content: []conversation.Content{conversation.Text("done")}},
	}}

	items := buildTranscriptFromConversation(conv)
	if len(items) != 4 {
		t.Fatalf("expected 4 transcript items, got %d", len(items))
	}
	if items[0].Prefix != "user" || items[0].Text != "hello" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Prefix != "assistant requested tool" {
		t.Fatalf("unexpected tool request item: %#v", items[1])
	}
	if items[2].Prefix != "tool" || items[2].Text != "/tmp/work" {
		t.Fatalf("unexpected tool response item: %#v", items[2])
	}
	if items[3].Prefix != "assistant" || items[3].Text != "done" {
		t.Fatalf("unexpected assistant item: %#v", items[3])
	}
}

func TestApplyAgentEventStreamsAssistantWithoutDuplicateFinalText(t *testing.T) {
	m := newModel(context.Background(), fakeRuntime{}, Options{})

	m.applyAgentEvent(agent.Event{Type: agent.EventTypeProviderTextDelta, Delta: "pon"})
	m.applyAgentEvent(agent.Event{Type: agent.EventTypeProviderTextDelta, Delta: "g"})
	if got := len(m.items); got != 1 {
		t.Fatalf("expected one live buffer item, got %d", got)
	}
	if m.items[0].Kind != kindLiveBuffer || m.items[0].Text != "pong" {
		t.Fatalf("unexpected live buffer item: %#v", m.items[0])
	}

	msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("pong"))
	m.applyAgentEvent(agent.Event{Type: agent.EventTypeAssistantMessageComplete, Message: &msg})
	if got := len(m.items); got != 1 {
		t.Fatalf("expected one finalized assistant item, got %d", got)
	}
	if m.items[0].Kind != kindAssistant || m.items[0].Text != "pong" {
		t.Fatalf("unexpected final assistant item: %#v", m.items[0])
	}
}
