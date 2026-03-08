package app

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

func TestRunAgentAutoModeRendersTranscript(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				if hasToolResponse(req.Messages) {
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("done"))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				}
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"printf hello"}`)))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader(""), &out, "say hello", RunOptions{WorkingDir: t.TempDir(), DBPath: t.TempDir() + "/sessions.db"})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}

	got := out.String()
	for _, want := range []string{"session:", "user> say hello", "assistant requested tool shell", "tool[shell]> hello", "assistant> done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got %q", want, got)
		}
	}
}

func TestRunAgentApproveModePrompts(t *testing.T) {
	originalProviderFactory := newRunProvider
	originalStoreOpener := openRunStore
	t.Cleanup(func() {
		newRunProvider = originalProviderFactory
		openRunStore = originalStoreOpener
	})

	newRunProvider = func(_ io.Writer) (provider.Provider, error) {
		return scriptedAppProvider{
			respond: func(req provider.Request) []provider.Event {
				if hasToolResponse(req.Messages) {
					msg := conversation.NewMessage(conversation.RoleAssistant, conversation.Text("understood"))
					return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
				}
				msg := conversation.NewMessage(conversation.RoleAssistant, conversation.ToolRequest("call_1", "shell", []byte(`{"command":"pwd"}`)))
				return []provider.Event{{Type: provider.EventTypeMessageComplete, Message: &msg}, {Type: provider.EventTypeDone}}
			},
		}, nil
	}

	var out bytes.Buffer
	err := RunAgent(context.Background(), strings.NewReader("n\n"), &out, "run pwd", RunOptions{Approve: true, WorkingDir: t.TempDir(), DBPath: t.TempDir() + "/sessions.db"})
	if err != nil {
		t.Fatalf("run agent: %v", err)
	}
	got := out.String()
	for _, want := range []string{"approve tool shell", "tool[shell]> tool execution denied by user", "assistant> understood"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got %q", want, got)
		}
	}
}

type scriptedAppProvider struct {
	respond func(provider.Request) []provider.Event
}

func (s scriptedAppProvider) streamWithRequest(req provider.Request) (<-chan provider.Event, error) {
	ch := make(chan provider.Event, len(s.respond(req)))
	for _, event := range s.respond(req) {
		ch <- event
	}
	close(ch)
	return ch, nil
}

func (s scriptedAppProvider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	_ = ctx
	return s.streamWithRequest(req)
}

func hasToolResponse(messages []conversation.Message) bool {
	for _, message := range messages {
		for _, content := range message.Content {
			if content.Type == conversation.ContentTypeToolResponse {
				return true
			}
		}
	}
	return false
}
