package openaicodex

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	codexauth "goose-go/internal/auth/codex"
	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

func TestBuildRequestBody(t *testing.T) {
	temp := 0.2
	req := provider.Request{
		SessionID:    "sess_123",
		SystemPrompt: "You are helpful.",
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("hello")),
			conversation.NewMessage(
				conversation.RoleAssistant,
				conversation.Text("working on it"),
				conversation.ToolRequestWithProviderID("call_1", "fc_1", "shell", json.RawMessage(`{"command":"pwd"}`)),
			),
		},
		Tools: []provider.ToolDefinition{
			{Name: "shell", Description: "run shell", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
		Model: provider.ModelConfig{
			Provider:        "openai-codex",
			Model:           "gpt-5-codex",
			Temperature:     &temp,
			MaxOutputTokens: 256,
		},
	}

	body, err := buildRequestBody(req)
	if err != nil {
		t.Fatalf("build request body: %v", err)
	}

	if body.Model != "gpt-5-codex" {
		t.Fatalf("expected model gpt-5-codex, got %q", body.Model)
	}
	if body.Instructions != "You are helpful." {
		t.Fatalf("expected system prompt")
	}
	if body.PromptCacheKey != "sess_123" {
		t.Fatalf("expected prompt cache key")
	}
	if len(body.Input) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(body.Input))
	}
	if body.Input[2].ID != "fc_1" {
		t.Fatalf("expected function call item id fc_1, got %q", body.Input[2].ID)
	}
	if body.Input[2].CallID != "call_1" {
		t.Fatalf("expected function call call_id call_1, got %q", body.Input[2].CallID)
	}
	if len(body.Tools) != 1 || body.Tools[0].Name != "shell" {
		t.Fatalf("expected one tool definition")
	}
}

func TestBuildHeaders(t *testing.T) {
	headers := buildHeaders(codexauth.Credentials{
		AccessToken: "access",
		AccountID:   "acct_123",
	})

	if headers["Authorization"] != "Bearer access" {
		t.Fatalf("unexpected auth header: %q", headers["Authorization"])
	}
	if headers["chatgpt-account-id"] != "acct_123" {
		t.Fatalf("unexpected account header")
	}
}

func TestNewProviderDoesNotSetClientTimeout(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	if p.client == nil {
		t.Fatal("expected provider client")
	}
	if p.client.Timeout != 0 {
		t.Fatalf("expected no default client timeout, got %s", p.client.Timeout)
	}
	transport, ok := p.client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected http.Transport, got %T", p.client.Transport)
	}
	if transport.ResponseHeaderTimeout != defaultResponseHeaderTimeout {
		t.Fatalf("expected response header timeout %s, got %s", defaultResponseHeaderTimeout, transport.ResponseHeaderTimeout)
	}
}

func TestProcessStream(t *testing.T) {
	stream := "" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hel\"}\n\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello\"}]}}\n\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"id\":\"fc_1\",\"call_id\":\"call_1\",\"name\":\"shell\",\"arguments\":\"{\\\"command\\\":\\\"pwd\\\"}\"}}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5,\"total_tokens\":15,\"input_tokens_details\":{\"cached_tokens\":2}}}}\n\n"

	events := make(chan provider.Event, 10)
	if err := processStream(strings.NewReader(stream), events, nil); err != nil {
		t.Fatalf("process stream: %v", err)
	}
	close(events)

	var got []provider.Event
	for event := range events {
		got = append(got, event)
	}

	if len(got) != 4 {
		t.Fatalf("expected 4 events, got %d", len(got))
	}
	if got[0].Type != provider.EventTypeTextDelta || got[0].Delta != "hel" {
		t.Fatalf("unexpected first event: %#v", got[0])
	}
	if got[1].Type != provider.EventTypeUsage || got[1].Usage.InputTokens != 8 {
		t.Fatalf("unexpected usage event: %#v", got[1])
	}
	if got[2].Type != provider.EventTypeMessageComplete {
		t.Fatalf("expected message complete event")
	}
	if len(got[2].Message.Content) != 2 {
		t.Fatalf("expected text + tool request in final message")
	}
	if got[2].Message.Content[1].ToolRequest.ProviderID != "fc_1" {
		t.Fatalf("expected provider id fc_1, got %#v", got[2].Message.Content[1].ToolRequest)
	}
	if got[3].Type != provider.EventTypeDone {
		t.Fatalf("expected done event")
	}
}

func TestStreamDebugOutput(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	expiresAt := time.Now().Add(time.Hour)
	secretToken := testJWT(t, expiresAt, "client_live", "acct_live")
	if err := os.WriteFile(authPath, []byte(authFixtureWithToken(t, secretToken)), 0o600); err != nil {
		t.Fatalf("write auth fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"pong\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"pong\"}]}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":4,\"output_tokens\":1,\"total_tokens\":5,\"input_tokens_details\":{\"cached_tokens\":0}}}}\n\n")
	}))
	defer server.Close()

	reader, err := codexauth.NewReader(
		codexauth.WithAuthPath(authPath),
		codexauth.WithNow(func() time.Time { return time.Now().UTC() }),
	)
	if err != nil {
		t.Fatalf("new auth reader: %v", err)
	}

	var debug bytes.Buffer
	p, err := New(
		WithAuthReader(reader),
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithDebugWriter(&debug),
	)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	events, err := p.Stream(context.Background(), provider.Request{
		SystemPrompt: "You are helpful.",
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("reply with pong")),
		},
		Model: provider.ModelConfig{
			Provider: "openai-codex",
			Model:    "gpt-5-codex",
		},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	for range events {
	}

	got := debug.String()
	for _, want := range []string{
		"=== normalized request ===",
		"=== codex request body ===",
		"=== codex request headers ===",
		"=== raw sse event ===",
		"=== normalized event ===",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected debug output to contain %q, got %q", want, got)
		}
	}
	if strings.Contains(got, secretToken) {
		t.Fatalf("expected authorization header to be redacted, got %q", got)
	}
}

func TestStream(t *testing.T) {
	authPath := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(authPath, []byte(authFixture(t, time.Now().Add(time.Hour))), 0o600); err != nil {
		t.Fatalf("write auth fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatalf("missing authorization header")
		}
		if got := r.Header.Get("chatgpt-account-id"); got != "acct_live" {
			t.Fatalf("unexpected account id %q", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["model"] != "gpt-5-codex" {
			t.Fatalf("expected model in request body")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"pong\"}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"pong\"}]}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":4,\"output_tokens\":1,\"total_tokens\":5,\"input_tokens_details\":{\"cached_tokens\":0}}}}\n\n")
	}))
	defer server.Close()

	reader, err := codexauth.NewReader(
		codexauth.WithAuthPath(authPath),
		codexauth.WithNow(func() time.Time { return time.Now().UTC() }),
	)
	if err != nil {
		t.Fatalf("new auth reader: %v", err)
	}

	p, err := New(
		WithAuthReader(reader),
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
	)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	events, err := p.Stream(context.Background(), provider.Request{
		SystemPrompt: "You are helpful.",
		Messages: []conversation.Message{
			conversation.NewMessage(conversation.RoleUser, conversation.Text("reply with pong")),
		},
		Model: provider.ModelConfig{
			Provider: "openai-codex",
			Model:    "gpt-5-codex",
		},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}

	var sawDone bool
	var sawMessage bool
	for event := range events {
		if event.Type == provider.EventTypeError {
			t.Fatalf("unexpected provider error: %v", event.Err)
		}
		if event.Type == provider.EventTypeMessageComplete {
			sawMessage = true
		}
		if event.Type == provider.EventTypeDone {
			sawDone = true
		}
	}

	if !sawMessage || !sawDone {
		t.Fatalf("expected message complete and done events")
	}
}

func authFixture(t *testing.T, expiresAt time.Time) string {
	t.Helper()
	return authFixtureWithToken(t, testJWT(t, expiresAt, "client_live", "acct_live"))
}

func authFixtureWithToken(t *testing.T, accessToken string) string {
	t.Helper()
	payload := map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh_live",
			"account_id":    "acct_live",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal auth fixture: %v", err)
	}
	return string(data)
}

func testJWT(t *testing.T, expiresAt time.Time, clientID, accountID string) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]any{
		"exp":       expiresAt.Unix(),
		"client_id": clientID,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
		},
	})
	if err != nil {
		t.Fatalf("marshal jwt payload: %v", err)
	}

	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}
