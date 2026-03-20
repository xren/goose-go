package codex

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	defaultClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	defaultAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	defaultRedirectURL  = "http://localhost:1455/auth/callback"
	defaultScope        = "openid profile email offline_access"
	defaultOriginator   = "pi"
	defaultLoginWait    = 60 * time.Second
	successHTML         = "<!doctype html><html><body><p>Authentication successful. Return to your terminal.</p></body></html>"
)

type LoginOption func(*loginFlow)

type loginFlow struct {
	authPath        string
	authorizeURL    string
	tokenURL        string
	redirectURL     string
	clientID        string
	scope           string
	originator      string
	client          *http.Client
	now             func() time.Time
	random          io.Reader
	locker          lockManager
	openBrowser     browserOpener
	callbackFactory callbackServerFactory
	waitTimeout     time.Duration
}

type browserOpener func(string) error

type callbackServerFactory func(state string, redirect *url.URL) (callbackServer, error)

type callbackServer interface {
	Wait(context.Context) (string, error)
	Close() error
}

func Login(ctx context.Context, in io.Reader, out io.Writer, opts ...LoginOption) error {
	authPath, err := writableAuthPath()
	if err != nil {
		return err
	}

	flow := &loginFlow{
		authPath:        authPath,
		authorizeURL:    defaultAuthorizeURL,
		tokenURL:        defaultTokenURL,
		redirectURL:     defaultRedirectURL,
		clientID:        defaultClientID,
		scope:           defaultScope,
		originator:      defaultOriginator,
		client:          http.DefaultClient,
		now:             func() time.Time { return time.Now().UTC() },
		random:          rand.Reader,
		locker:          fileLockManager{now: func() time.Time { return time.Now().UTC() }, sleep: time.Sleep, staleAfter: defaultLockTTL, poll: defaultLockPoll},
		openBrowser:     openBrowser,
		callbackFactory: newLocalCallbackServer,
		waitTimeout:     defaultLoginWait,
	}

	for _, opt := range opts {
		opt(flow)
	}

	return flow.run(ctx, in, out)
}

func WithLoginAuthPath(path string) LoginOption {
	return func(f *loginFlow) {
		f.authPath = path
	}
}

func WithAuthorizeURL(rawURL string) LoginOption {
	return func(f *loginFlow) {
		f.authorizeURL = rawURL
	}
}

func WithLoginTokenURL(rawURL string) LoginOption {
	return func(f *loginFlow) {
		f.tokenURL = rawURL
	}
}

func WithRedirectURL(rawURL string) LoginOption {
	return func(f *loginFlow) {
		f.redirectURL = rawURL
	}
}

func WithLoginHTTPClient(client *http.Client) LoginOption {
	return func(f *loginFlow) {
		f.client = client
	}
}

func WithLoginNow(now func() time.Time) LoginOption {
	return func(f *loginFlow) {
		f.now = now
	}
}

func WithRandomReader(reader io.Reader) LoginOption {
	return func(f *loginFlow) {
		f.random = reader
	}
}

func WithBrowserOpener(opener browserOpener) LoginOption {
	return func(f *loginFlow) {
		f.openBrowser = opener
	}
}

func WithCallbackServerFactory(factory callbackServerFactory) LoginOption {
	return func(f *loginFlow) {
		f.callbackFactory = factory
	}
}

func WithLoginLockManager(locker lockManager) LoginOption {
	return func(f *loginFlow) {
		f.locker = locker
	}
}

func WithLoginWaitTimeout(timeout time.Duration) LoginOption {
	return func(f *loginFlow) {
		f.waitTimeout = timeout
	}
}

func (f *loginFlow) run(ctx context.Context, in io.Reader, out io.Writer) error {
	redirectURL, err := url.Parse(f.redirectURL)
	if err != nil {
		return fmt.Errorf("parse redirect url: %w", err)
	}

	verifier, challenge, err := generatePKCE(f.random)
	if err != nil {
		return err
	}
	state, err := randomHex(f.random, 16)
	if err != nil {
		return err
	}
	authURL, err := f.authorizationURL(challenge, state)
	if err != nil {
		return err
	}

	var server callbackServer
	if f.callbackFactory != nil {
		server, err = f.callbackFactory(state, redirectURL)
		if err != nil {
			_, _ = fmt.Fprintf(out, "browser callback unavailable (%v); falling back to manual paste\n", err)
		}
	}
	if server != nil {
		defer func() { _ = server.Close() }()
	}

	_, _ = fmt.Fprintf(out, "Open this URL to authenticate:\n%s\n", authURL)
	if f.openBrowser != nil {
		if err := f.openBrowser(authURL); err != nil {
			_, _ = fmt.Fprintf(out, "could not open browser automatically: %v\n", err)
		} else {
			_, _ = fmt.Fprintln(out, "opened browser for authentication")
		}
	}

	code, err := f.obtainAuthorizationCode(ctx, in, out, server, state)
	if err != nil {
		return err
	}

	tokenResp, err := f.exchangeAuthorizationCode(ctx, code, verifier)
	if err != nil {
		return err
	}

	accountID, clientID, err := credentialsFromAccessToken(tokenResp.AccessToken)
	if err != nil {
		return err
	}
	if accountID == "" {
		return errors.New("failed to extract account_id from access token")
	}

	creds := Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		AccountID:    accountID,
		ClientID:     clientID,
		ExpiresAt:    f.now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).UTC(),
		LastRefresh:  f.now().UTC(),
		AuthMode:     defaultAuthMode,
	}

	if err := f.storeCredentials(ctx, creds, tokenResp.IDToken); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "saved Codex credentials to %s\n", f.authPath)
	return nil
}

func (f *loginFlow) authorizationURL(challenge string, state string) (string, error) {
	authURL, err := url.Parse(f.authorizeURL)
	if err != nil {
		return "", fmt.Errorf("parse authorize url: %w", err)
	}
	authURL.RawQuery = url.Values{
		"response_type":              []string{"code"},
		"client_id":                  []string{f.clientID},
		"redirect_uri":               []string{f.redirectURL},
		"scope":                      []string{f.scope},
		"code_challenge":             []string{challenge},
		"code_challenge_method":      []string{"S256"},
		"state":                      []string{state},
		"id_token_add_organizations": []string{"true"},
		"codex_cli_simplified_flow":  []string{"true"},
		"originator":                 []string{f.originator},
	}.Encode()
	return authURL.String(), nil
}

func (f *loginFlow) obtainAuthorizationCode(ctx context.Context, in io.Reader, out io.Writer, server callbackServer, state string) (string, error) {
	if server != nil {
		_, _ = fmt.Fprintf(out, "waiting for browser callback on %s\n", f.redirectURL)
		waitCtx, cancel := context.WithTimeout(ctx, f.waitTimeout)
		defer cancel()
		code, err := server.Wait(waitCtx)
		if err == nil && code != "" {
			return code, nil
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			_, _ = fmt.Fprintf(out, "browser callback failed: %v\n", err)
		}
		_, _ = fmt.Fprintln(out, "browser callback did not complete in time; use manual paste")
	}

	_, _ = fmt.Fprint(out, "Paste the authorization code or full redirect URL: ")
	input, err := readLine(in)
	if err != nil {
		return "", fmt.Errorf("read authorization input: %w", err)
	}
	code, parsedState, err := parseAuthorizationInput(input)
	if err != nil {
		return "", err
	}
	if parsedState != "" && parsedState != state {
		return "", errors.New("state mismatch")
	}
	if code == "" {
		return "", errors.New("missing authorization code")
	}
	return code, nil
}

func (f *loginFlow) exchangeAuthorizationCode(ctx context.Context, code string, verifier string) (tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", f.clientID)
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", f.redirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("build token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := f.client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return tokenResponse{}, fmt.Errorf("exchange authorization code: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return tokenResponse{}, fmt.Errorf("decode token exchange response: %w", err)
	}
	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" || tokenResp.ExpiresIn <= 0 {
		return tokenResponse{}, errors.New("token exchange response missing required fields")
	}
	return tokenResp, nil
}

func (f *loginFlow) storeCredentials(ctx context.Context, creds Credentials, idToken string) error {
	if f.locker == nil {
		return errors.New("codex auth lock manager is required")
	}
	unlock, err := f.locker.Lock(ctx, f.authPath+".lock")
	if err != nil {
		return fmt.Errorf("lock codex auth file: %w", err)
	}
	defer unlock()

	file := authFile{AuthMode: defaultAuthMode}
	if data, err := os.ReadFile(f.authPath); err == nil {
		if err := json.Unmarshal(data, &file); err != nil {
			return fmt.Errorf("decode existing codex auth file: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing codex auth file: %w", err)
	}

	file.AuthMode = defaultAuthMode
	file.LastRefresh = creds.LastRefresh.UTC().Format(time.RFC3339)
	file.Tokens.AccessToken = creds.AccessToken
	file.Tokens.RefreshToken = creds.RefreshToken
	file.Tokens.AccountID = creds.AccountID
	if idToken != "" {
		file.Tokens.IDToken = idToken
	}

	if err := writeAuthFileAtomically(f.authPath, file); err != nil {
		return err
	}
	return nil
}

func generatePKCE(random io.Reader) (string, string, error) {
	verifier, err := randomBase64URL(random, 32)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomHex(random io.Reader, size int) (string, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(random, buf); err != nil {
		return "", fmt.Errorf("generate random state: %w", err)
	}
	return fmt.Sprintf("%x", buf), nil
}

func randomBase64URL(random io.Reader, size int) (string, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(random, buf); err != nil {
		return "", fmt.Errorf("generate pkce verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func parseAuthorizationInput(input string) (string, string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", nil
	}
	if parsedURL, err := url.Parse(value); err == nil && parsedURL.Scheme != "" && parsedURL.Host != "" {
		return parsedURL.Query().Get("code"), parsedURL.Query().Get("state"), nil
	}
	if strings.Contains(value, "code=") {
		params, err := url.ParseQuery(value)
		if err != nil {
			return "", "", fmt.Errorf("parse authorization input: %w", err)
		}
		return params.Get("code"), params.Get("state"), nil
	}
	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		code := parts[0]
		state := ""
		if len(parts) > 1 {
			state = parts[1]
		}
		return strings.TrimSpace(code), strings.TrimSpace(state), nil
	}
	return value, "", nil
}

func credentialsFromAccessToken(token string) (string, string, error) {
	claims, err := parseTokenClaims(token)
	if err != nil {
		return "", "", fmt.Errorf("parse access token: %w", err)
	}
	return claims.Auth.ChatGPTAccountID, claims.ClientID, nil
}

func readLine(in io.Reader) (string, error) {
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

type localCallbackServer struct {
	server *http.Server
	codeCh chan string
}

func newLocalCallbackServer(state string, redirect *url.URL) (callbackServer, error) {
	listenAddr := redirect.Host
	if redirect.Hostname() == "localhost" {
		listenAddr = net.JoinHostPort("127.0.0.1", redirect.Port())
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}

	codeCh := make(chan string, 1)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != redirect.Path {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			return
		}
		select {
		case codeCh <- code:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, successHTML)
	})

	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()

	return &localCallbackServer{server: server, codeCh: codeCh}, nil
}

func (s *localCallbackServer) Wait(ctx context.Context) (string, error) {
	select {
	case code := <-s.codeCh:
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *localCallbackServer) Close() error {
	if s == nil || s.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func openBrowser(rawURL string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	default:
		return exec.Command("xdg-open", rawURL).Start()
	}
}
