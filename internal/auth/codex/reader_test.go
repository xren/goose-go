package codex

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveReadsExistingCredentials(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	path := filepath.Join(t.TempDir(), "auth.json")
	writeFixture(t, path, authFixture{
		AuthMode:    "chatgpt",
		LastRefresh: now.Add(-time.Hour).Format(time.RFC3339),
		Tokens: authFixtureTokens{
			AccessToken:  makeJWT(t, jwtFixture{Exp: now.Add(time.Hour), ClientID: "client_123", AccountID: "acct_123"}),
			RefreshToken: "refresh_123",
			IDToken:      makeJWT(t, jwtFixture{Exp: now.Add(time.Hour), AccountID: "acct_123"}),
		},
	})

	reader, err := NewReader(
		WithAuthPath(path),
		WithNow(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	creds, err := reader.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if creds.AccountID != "acct_123" {
		t.Fatalf("expected account id acct_123, got %q", creds.AccountID)
	}
	if creds.ClientID != "client_123" {
		t.Fatalf("expected client id client_123, got %q", creds.ClientID)
	}
}

func TestResolveDerivesAccountIDFromTokenClaims(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	path := filepath.Join(t.TempDir(), "auth.json")
	writeFixture(t, path, authFixture{
		AuthMode: "chatgpt",
		Tokens: authFixtureTokens{
			AccessToken:  makeJWT(t, jwtFixture{Exp: now.Add(time.Hour), ClientID: "client_123", AccountID: "acct_from_claim"}),
			RefreshToken: "refresh_123",
		},
	})

	reader, err := NewReader(
		WithAuthPath(path),
		WithNow(func() time.Time { return now }),
	)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	creds, err := reader.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if creds.AccountID != "acct_from_claim" {
		t.Fatalf("expected derived account id, got %q", creds.AccountID)
	}
}

func TestResolveRefreshesExpiredTokenAndPreservesFields(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	path := filepath.Join(t.TempDir(), "auth.json")
	writeFixture(t, path, authFixture{
		OpenAIAPIKey: "should-stay",
		AuthMode:     "chatgpt",
		LastRefresh:  now.Add(-2 * time.Hour).Format(time.RFC3339),
		Custom:       map[string]any{"custom_root": map[string]any{"nested": true}},
		Tokens: authFixtureTokens{
			AccessToken:  makeJWT(t, jwtFixture{Exp: now.Add(2 * time.Minute), ClientID: "client_123", AccountID: "acct_old"}),
			RefreshToken: "refresh_old",
			IDToken:      "id_old",
			Custom:       map[string]any{"custom_token": "keep-me"},
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Fatalf("expected refresh_token grant, got %q", got)
		}
		if got := r.Form.Get("refresh_token"); got != "refresh_old" {
			t.Fatalf("expected refresh_old, got %q", got)
		}
		if got := r.Form.Get("client_id"); got != "client_123" {
			t.Fatalf("expected client_123, got %q", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  makeJWT(t, jwtFixture{Exp: now.Add(time.Hour), ClientID: "client_123", AccountID: "acct_new"}),
			"refresh_token": "refresh_new",
			"id_token":      "id_new",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	reader, err := NewReader(
		WithAuthPath(path),
		WithNow(func() time.Time { return now }),
		WithTokenURL(server.URL),
		WithHTTPClient(server.Client()),
	)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	creds, err := reader.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	if creds.AccountID != "acct_new" {
		t.Fatalf("expected refreshed account id acct_new, got %q", creds.AccountID)
	}
	if creds.RefreshToken != "refresh_new" {
		t.Fatalf("expected refresh_new, got %q", creds.RefreshToken)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated auth file: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decode updated auth file: %v", err)
	}

	if raw["OPENAI_API_KEY"] != "should-stay" {
		t.Fatalf("expected OPENAI_API_KEY to be preserved")
	}
	if _, ok := raw["custom_root"]; !ok {
		t.Fatalf("expected custom_root to be preserved")
	}

	tokens, ok := raw["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("expected tokens object")
	}
	if tokens["custom_token"] != "keep-me" {
		t.Fatalf("expected custom token field to be preserved")
	}
	if tokens["refresh_token"] != "refresh_new" {
		t.Fatalf("expected refresh token to be updated")
	}
}

func TestResolveRejectsMissingAuthFile(t *testing.T) {
	reader, err := NewReader(WithAuthPath(filepath.Join(t.TempDir(), "missing.json")))
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}

	_, err = reader.Resolve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "codex login") {
		t.Fatalf("expected actionable login error, got %v", err)
	}
}

func TestDefaultAuthPathUsesEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv(AuthFileEnvVar, path)

	got, err := defaultAuthPath()
	if err != nil {
		t.Fatalf("default auth path: %v", err)
	}

	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

type authFixture struct {
	OpenAIAPIKey string
	AuthMode     string
	LastRefresh  string
	Tokens       authFixtureTokens
	Custom       map[string]any
}

type authFixtureTokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	AccountID    string
	Custom       map[string]any
}

type jwtFixture struct {
	Exp       time.Time
	ClientID  string
	AccountID string
}

func writeFixture(t *testing.T, path string, fixture authFixture) {
	t.Helper()

	root := map[string]any{}
	if fixture.OpenAIAPIKey != "" {
		root["OPENAI_API_KEY"] = fixture.OpenAIAPIKey
	}
	if fixture.AuthMode != "" {
		root["auth_mode"] = fixture.AuthMode
	}
	if fixture.LastRefresh != "" {
		root["last_refresh"] = fixture.LastRefresh
	}
	for key, value := range fixture.Custom {
		root[key] = value
	}

	tokens := map[string]any{
		"access_token":  fixture.Tokens.AccessToken,
		"refresh_token": fixture.Tokens.RefreshToken,
	}
	if fixture.Tokens.IDToken != "" {
		tokens["id_token"] = fixture.Tokens.IDToken
	}
	if fixture.Tokens.AccountID != "" {
		tokens["account_id"] = fixture.Tokens.AccountID
	}
	for key, value := range fixture.Tokens.Custom {
		tokens[key] = value
	}
	root["tokens"] = tokens

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func makeJWT(t *testing.T, fixture jwtFixture) string {
	t.Helper()

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))

	payloadValue := map[string]any{
		"exp": fixture.Exp.Unix(),
		authClaimKey: map[string]any{
			"chatgpt_account_id": fixture.AccountID,
		},
	}
	if fixture.ClientID != "" {
		payloadValue["client_id"] = fixture.ClientID
	}

	payloadBytes, err := json.Marshal(payloadValue)
	if err != nil {
		t.Fatalf("marshal jwt payload: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	return header + "." + payload + ".signature"
}
