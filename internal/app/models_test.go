package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestListModelsPrintsRegistryEntries(t *testing.T) {
	var out bytes.Buffer
	if err := ListModels(context.Background(), &out, RunOptions{Provider: "openai-codex"}); err != nil {
		t.Fatalf("list models: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "gpt-5-codex") {
		t.Fatalf("expected default model in output, got %q", got)
	}
}
