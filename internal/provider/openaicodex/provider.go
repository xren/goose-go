package openaicodex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	codexauth "goose-go/internal/auth/codex"
	"goose-go/internal/conversation"
	"goose-go/internal/provider"
)

const (
	defaultBaseURL               = "https://chatgpt.com/backend-api"
	defaultResponseHeaderTimeout = 2 * time.Minute
)

type Provider struct {
	authReader *codexauth.Reader
	baseURL    string
	client     *http.Client
	debugOut   io.Writer
}

type Option func(*Provider)

type requestBody struct {
	Model             string           `json:"model"`
	Store             bool             `json:"store"`
	Stream            bool             `json:"stream"`
	Instructions      string           `json:"instructions,omitempty"`
	Input             []inputItem      `json:"input,omitempty"`
	Tools             []toolDefinition `json:"tools,omitempty"`
	ToolChoice        string           `json:"tool_choice,omitempty"`
	ParallelToolCalls bool             `json:"parallel_tool_calls,omitempty"`
	Temperature       *float64         `json:"temperature,omitempty"`
	MaxOutputTokens   int              `json:"max_output_tokens,omitempty"`
	PromptCacheKey    string           `json:"prompt_cache_key,omitempty"`
}

type inputItem struct {
	Type      string            `json:"type,omitempty"`
	Role      string            `json:"role,omitempty"`
	Content   any               `json:"content,omitempty"`
	ID        string            `json:"id,omitempty"`
	Status    string            `json:"status,omitempty"`
	CallID    string            `json:"call_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments string            `json:"arguments,omitempty"`
	Output    string            `json:"output,omitempty"`
	Phase     string            `json:"phase,omitempty"`
	Extra     map[string]string `json:"-"`
}

type toolDefinition struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type sseEvent struct {
	Type      string          `json:"type"`
	Delta     string          `json:"delta,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Code      string          `json:"code,omitempty"`
	Message   string          `json:"message,omitempty"`
	Item      json.RawMessage `json:"item,omitempty"`
	Response  struct {
		Status string `json:"status,omitempty"`
		Usage  struct {
			InputTokens        int `json:"input_tokens"`
			OutputTokens       int `json:"output_tokens"`
			TotalTokens        int `json:"total_tokens"`
			InputTokensDetails struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"input_tokens_details"`
		} `json:"usage"`
	} `json:"response,omitempty"`
}

type responseMessageItem struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Role    string `json:"role"`
	Status  string `json:"status"`
	Content []struct {
		Type    string `json:"type"`
		Text    string `json:"text,omitempty"`
		Refusal string `json:"refusal,omitempty"`
	} `json:"content"`
}

type responseFunctionCallItem struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

var _ provider.Provider = (*Provider)(nil)

func New(opts ...Option) (*Provider, error) {
	authReader, err := codexauth.NewReader()
	if err != nil {
		return nil, err
	}

	p := &Provider{
		authReader: authReader,
		baseURL:    defaultBaseURL,
		// Streaming responses should not use a whole-request timeout because SSE
		// streams can legitimately run for a long time. Bound only the wait for
		// initial response headers so stalled requests fail instead of hanging.
		client: &http.Client{Transport: newDefaultTransport()},
	}

	for _, opt := range opts {
		opt(p)
	}

	return p, nil
}

func WithBaseURL(baseURL string) Option {
	return func(p *Provider) {
		p.baseURL = baseURL
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) {
		p.client = client
	}
}

func WithAuthReader(reader *codexauth.Reader) Option {
	return func(p *Provider) {
		p.authReader = reader
	}
}

func WithDebugWriter(w io.Writer) Option {
	return func(p *Provider) {
		p.debugOut = w
	}
}

func newDefaultTransport() http.RoundTripper {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}
	clone := base.Clone()
	clone.ResponseHeaderTimeout = defaultResponseHeaderTimeout
	return clone
}

func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if p.authReader == nil {
		return nil, errors.New("codex auth reader is required")
	}

	creds, err := p.authReader.Resolve(ctx)
	if err != nil {
		return nil, err
	}

	debugPretty(p.debugOut, "normalized request", req)

	body, err := buildRequestBody(req)
	if err != nil {
		return nil, err
	}

	debugPretty(p.debugOut, "codex request body", body)

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, resolveURL(p.baseURL), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build codex request: %w", err)
	}

	headers := buildHeaders(creds)
	debugPretty(p.debugOut, "codex request headers", redactHeaders(headers))

	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send codex request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("codex request failed: status %d: %s", resp.StatusCode, string(body))
	}

	events := make(chan provider.Event)
	go func() {
		defer close(events)
		defer func() { _ = resp.Body.Close() }()

		if err := processStream(resp.Body, events, p.debugOut); err != nil {
			events <- provider.Event{Type: provider.EventTypeError, Err: err}
		}
	}()

	return events, nil
}

func buildRequestBody(req provider.Request) (requestBody, error) {
	input, err := buildInput(req.Messages)
	if err != nil {
		return requestBody{}, err
	}

	body := requestBody{
		Model:             req.Model.Model,
		Store:             false,
		Stream:            true,
		Instructions:      req.SystemPrompt,
		Input:             input,
		ToolChoice:        "auto",
		ParallelToolCalls: true,
		Temperature:       req.Model.Temperature,
		MaxOutputTokens:   req.Model.MaxOutputTokens,
		PromptCacheKey:    req.SessionID,
	}

	if len(req.Tools) > 0 {
		body.Tools = make([]toolDefinition, 0, len(req.Tools))
		for _, tool := range req.Tools {
			body.Tools = append(body.Tools, toolDefinition{
				Type:        "function",
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			})
		}
	}

	return body, nil
}

func buildInput(messages []conversation.Message) ([]inputItem, error) {
	var items []inputItem

	for _, message := range messages {
		switch message.Role {
		case conversation.RoleUser:
			text := joinTextContent(message.Content)
			if text == "" {
				continue
			}
			items = append(items, inputItem{
				Role:    "user",
				Content: []map[string]string{{"type": "input_text", "text": text}},
			})
		case conversation.RoleSystem:
			text := joinTextContent(message.Content)
			if text == "" {
				continue
			}
			items = append(items, inputItem{
				Role:    "system",
				Content: text,
			})
		case conversation.RoleAssistant:
			for _, content := range message.Content {
				switch content.Type {
				case conversation.ContentTypeText:
					items = append(items, inputItem{
						Type:   "message",
						ID:     message.ID,
						Role:   "assistant",
						Status: "completed",
						Content: []map[string]string{{
							"type": "output_text",
							"text": content.Text.Text,
						}},
					})
				case conversation.ContentTypeToolRequest:
					items = append(items, inputItem{
						Type:      "function_call",
						ID:        normalizeFunctionCallItemID(*content.ToolRequest),
						CallID:    content.ToolRequest.ID,
						Name:      content.ToolRequest.Name,
						Arguments: compactJSON(content.ToolRequest.Arguments),
					})
				}
			}
		default:
			for _, content := range message.Content {
				if content.Type != conversation.ContentTypeToolResponse {
					continue
				}
				items = append(items, inputItem{
					Type:   "function_call_output",
					CallID: content.ToolResponse.ID,
					Output: joinToolResponseContent(*content.ToolResponse),
				})
			}
		}
	}

	return items, nil
}

func buildHeaders(creds codexauth.Credentials) map[string]string {
	return map[string]string{
		"Authorization":      "Bearer " + creds.AccessToken,
		"chatgpt-account-id": creds.AccountID,
		"OpenAI-Beta":        "responses=experimental",
		"accept":             "text/event-stream",
		"content-type":       "application/json",
	}
}

func resolveURL(baseURL string) string {
	normalized := strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasSuffix(normalized, "/codex/responses"):
		return normalized
	case strings.HasSuffix(normalized, "/codex"):
		return normalized + "/responses"
	default:
		return normalized + "/codex/responses"
	}
}

func processStream(body io.Reader, events chan<- provider.Event, debugOut io.Writer) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var dataLines []string
	var finalContent []conversation.Content

	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}

		payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = nil
		if payload == "" || payload == "[DONE]" {
			return nil
		}

		var event sseEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return fmt.Errorf("decode SSE event: %w", err)
		}

		debugPretty(debugOut, "raw sse event", event)

		switch event.Type {
		case "response.output_text.delta", "response.refusal.delta":
			if event.Delta != "" {
				normalized := provider.Event{Type: provider.EventTypeTextDelta, Delta: event.Delta}
				debugEvent(debugOut, normalized)
				events <- normalized
			}
		case "response.output_item.done":
			content, err := parseOutputItem(event.Item)
			if err != nil {
				return err
			}
			if content != nil {
				finalContent = append(finalContent, *content)
			}
		case "response.completed", "response.done":
			usage := &provider.Usage{
				InputTokens:  event.Response.Usage.InputTokens - event.Response.Usage.InputTokensDetails.CachedTokens,
				OutputTokens: event.Response.Usage.OutputTokens,
				TotalTokens:  event.Response.Usage.TotalTokens,
			}
			usageEvent := provider.Event{Type: provider.EventTypeUsage, Usage: usage}
			debugEvent(debugOut, usageEvent)
			events <- usageEvent
			if len(finalContent) > 0 {
				message := conversation.NewMessage(conversation.RoleAssistant, finalContent...)
				messageEvent := provider.Event{Type: provider.EventTypeMessageComplete, Message: &message}
				debugEvent(debugOut, messageEvent)
				events <- messageEvent
			}
			doneEvent := provider.Event{Type: provider.EventTypeDone}
			debugEvent(debugOut, doneEvent)
			events <- doneEvent
		case "error", "response.failed":
			return fmt.Errorf("codex error %s: %s", event.Code, event.Message)
		}

		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read SSE stream: %w", err)
	}

	return flush()
}

func parseOutputItem(raw json.RawMessage) (*conversation.Content, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var kind struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &kind); err != nil {
		return nil, fmt.Errorf("decode output item kind: %w", err)
	}

	switch kind.Type {
	case "message":
		var item responseMessageItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("decode output message item: %w", err)
		}
		var text strings.Builder
		for _, part := range item.Content {
			switch part.Type {
			case "output_text":
				text.WriteString(part.Text)
			case "refusal":
				text.WriteString(part.Refusal)
			}
		}
		if text.Len() == 0 {
			return nil, nil
		}
		content := conversation.Text(text.String())
		return &content, nil
	case "function_call":
		var item responseFunctionCallItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("decode output function call item: %w", err)
		}
		args := json.RawMessage(item.Arguments)
		if len(bytes.TrimSpace(args)) == 0 {
			args = json.RawMessage(`{}`)
		}
		content := conversation.ToolRequestWithProviderID(item.CallID, item.ID, item.Name, args)
		return &content, nil
	default:
		return nil, nil
	}
}

func normalizeFunctionCallItemID(content conversation.ToolRequestContent) string {
	if strings.HasPrefix(content.ProviderID, "fc") {
		return content.ProviderID
	}
	if strings.HasPrefix(content.ID, "fc") {
		return content.ID
	}
	return "fc_" + content.ID
}

func joinTextContent(contents []conversation.Content) string {
	parts := make([]string, 0, len(contents))
	for _, content := range contents {
		if content.Type == conversation.ContentTypeText && content.Text != nil && content.Text.Text != "" {
			parts = append(parts, content.Text.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func joinToolResponseContent(result conversation.ToolResponseContent) string {
	var parts []string
	for _, content := range result.Content {
		if content.Text != "" {
			parts = append(parts, content.Text)
		}
	}
	if len(parts) == 0 && len(bytes.TrimSpace(result.Structured)) > 0 {
		return string(result.Structured)
	}
	if len(parts) == 0 {
		return "(no output)"
	}
	return strings.Join(parts, "\n")
}

func compactJSON(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return "{}"
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}

	out, err := json.Marshal(value)
	if err != nil {
		return string(raw)
	}
	return string(out)
}

func debugPretty(w io.Writer, title string, value any) {
	if w == nil {
		return
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(w, "\n=== %s ===\n<marshal error: %v>\n", title, err)
		return
	}

	_, _ = fmt.Fprintf(w, "\n=== %s ===\n%s\n", title, data)
}

func debugEvent(w io.Writer, event provider.Event) {
	if w == nil {
		return
	}

	payload := map[string]any{
		"type": event.Type,
	}
	if event.Delta != "" {
		payload["delta"] = event.Delta
	}
	if event.Usage != nil {
		payload["usage"] = event.Usage
	}
	if event.Message != nil {
		payload["message"] = event.Message
	}
	if event.Err != nil {
		payload["error"] = event.Err.Error()
	}
	debugPretty(w, "normalized event", payload)
}

func redactHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	if token, ok := out["Authorization"]; ok && strings.HasPrefix(token, "Bearer ") {
		raw := strings.TrimPrefix(token, "Bearer ")
		out["Authorization"] = "Bearer " + redactToken(raw)
	}
	return out
}

func redactToken(token string) string {
	if len(token) <= 12 {
		return "REDACTED"
	}
	return token[:6] + "..." + token[len(token)-4:]
}
