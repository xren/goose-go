package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAuthorizationURLMatchesKnownGoodCodexFlow(t *testing.T) {
	flow := &loginFlow{
		authorizeURL: defaultAuthorizeURL,
		redirectURL:  defaultRedirectURL,
		clientID:     defaultClientID,
		scope:        defaultScope,
		originator:   defaultOriginator,
	}

	got, err := flow.authorizationURL("challenge_123", "state_123")
	if err != nil {
		t.Fatalf("authorization url: %v", err)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse authorization url: %v", err)
	}

	params := parsed.Query()
	if parsed.String() == "" {
		t.Fatal("expected non-empty url")
	}
	if params.Get("redirect_uri") != "http://localhost:1455/auth/callback" {
		t.Fatalf("expected localhost redirect uri, got %q", params.Get("redirect_uri"))
	}
	if params.Get("originator") != "pi" {
		t.Fatalf("expected originator pi, got %q", params.Get("originator"))
	}
	if params.Get("codex_cli_simplified_flow") != "true" {
		t.Fatalf("expected simplified flow flag, got %q", params.Get("codex_cli_simplified_flow"))
	}
}

func TestLoginWritesNativeAuthFileFromManualCode(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	authPath := filepath.Join(t.TempDir(), "auth.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "authorization_code" {
			t.Fatalf("expected authorization_code grant, got %q", got)
		}
		if got := r.Form.Get("code"); got != "manual_code" {
			t.Fatalf("expected manual_code, got %q", got)
		}
		if got := r.Form.Get("client_id"); got != defaultClientID {
			t.Fatalf("expected client id %q, got %q", defaultClientID, got)
		}
		if got := r.Form.Get("redirect_uri"); got != defaultRedirectURL {
			t.Fatalf("expected redirect uri %q, got %q", defaultRedirectURL, got)
		}
		if got := r.Form.Get("code_verifier"); got == "" {
			t.Fatal("expected code verifier")
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  makeJWT(t, jwtFixture{Exp: now.Add(time.Hour), ClientID: defaultClientID, AccountID: "acct_login"}),
			"refresh_token": "refresh_login",
			"id_token":      "id_login",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	var out bytes.Buffer
	err := Login(
		context.Background(),
		strings.NewReader("manual_code\n"),
		&out,
		WithLoginAuthPath(authPath),
		WithLoginTokenURL(server.URL),
		WithLoginNow(func() time.Time { return now }),
		WithRandomReader(bytes.NewReader(bytes.Repeat([]byte{1}, 64))),
		WithBrowserOpener(func(string) error { return nil }),
		WithCallbackServerFactory(func(state string, redirectURL *url.URL) (callbackServer, error) {
			return nil, errors.New("bind failed")
		}),
		WithLoginLockManager(stubLockManager{lock: func(ctx context.Context, path string) (func(), error) { return func() {}, nil }}),
	)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	reader, err := NewReader(
		WithAuthPath(authPath),
		WithNow(func() time.Time { return now }),
		WithLockManager(stubLockManager{lock: func(ctx context.Context, path string) (func(), error) { return func() {}, nil }}),
	)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	creds, err := reader.Resolve(context.Background())
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if creds.AccountID != "acct_login" || creds.RefreshToken != "refresh_login" {
		t.Fatalf("unexpected resolved credentials: %+v", creds)
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatalf("read auth file: %v", err)
	}
	if !strings.Contains(string(data), `"auth_mode": "chatgpt"`) {
		t.Fatalf("expected auth mode to be stored, got %s", string(data))
	}
}
