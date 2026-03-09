package fetchurl

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"goose-go/internal/conversation"
	"goose-go/internal/tools"
)

const defaultMaxBytes = 64 * 1024

type Tool struct {
	client *http.Client
}

type Arguments struct {
	URL      string `json:"url"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

type StructuredResult struct {
	URL         string `json:"url"`
	FinalURL    string `json:"final_url"`
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type"`
	ReadBytes   int    `json:"read_bytes"`
	Truncated   bool   `json:"truncated"`
}

func New() Tool {
	return Tool{
		client: &http.Client{Timeout: 20 * time.Second},
	}
}

func (Tool) Definition() tools.Definition {
	return tools.Definition{
		Name:            "fetch_url",
		Description:     "Fetch a web page or text resource over HTTP.",
		InputSchema:     json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"},"max_bytes":{"type":"integer","minimum":1}},"required":["url"],"additionalProperties":false}`),
		Capability:      tools.CapabilityRead,
		ApprovalDefault: tools.ApprovalDefaultAllow,
	}
}

func (t Tool) Run(ctx context.Context, call tools.Call) (tools.Result, error) {
	var args Arguments
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return tools.Result{}, fmt.Errorf("%w: decode fetch_url arguments: %v", tools.ErrInvalidToolCall, err)
	}
	if strings.TrimSpace(args.URL) == "" {
		return tools.Result{}, fmt.Errorf("%w: fetch_url url is required", tools.ErrInvalidToolCall)
	}
	if args.MaxBytes <= 0 {
		args.MaxBytes = defaultMaxBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, args.URL, nil)
	if err != nil {
		return tools.Result{}, fmt.Errorf("%w: invalid url: %v", tools.ErrInvalidToolCall, err)
	}
	req.Header.Set("User-Agent", "goose-go/fetch_url")

	resp, err := t.client.Do(req)
	if err != nil {
		return tools.Result{}, fmt.Errorf("fetch %s: %w", args.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	limit := int64(args.MaxBytes + 1)
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return tools.Result{}, fmt.Errorf("read response body: %w", err)
	}
	truncated := len(body) > args.MaxBytes
	if truncated {
		body = body[:args.MaxBytes]
	}

	contentType := resp.Header.Get("Content-Type")
	text := string(body)
	if strings.Contains(strings.ToLower(contentType), "html") {
		text = cleanHTML(text)
	}
	if truncated {
		text += "\n\n[truncated]"
	}

	structured, err := json.Marshal(StructuredResult{
		URL:         args.URL,
		FinalURL:    resp.Request.URL.String(),
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		ReadBytes:   len(body),
		Truncated:   truncated,
	})
	if err != nil {
		return tools.Result{}, fmt.Errorf("marshal fetch_url result: %w", err)
	}

	return tools.Result{
		ToolCallID: call.ID,
		IsError:    resp.StatusCode >= 400,
		Content:    []conversation.ToolResult{{Type: "text", Text: text}},
		Structured: structured,
	}, nil
}

var (
	scriptStyleRE = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	tagRE         = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRE       = regexp.MustCompile(`[ \t\r\f\v]+`)
)

func cleanHTML(body string) string {
	cleaned := scriptStyleRE.ReplaceAllString(body, " ")
	cleaned = tagRE.ReplaceAllString(cleaned, "\n")
	cleaned = html.UnescapeString(cleaned)
	lines := strings.Split(cleaned, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(spaceRE.ReplaceAllString(line, " "))
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
