package codex

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	AuthFileEnvVar  = "GOOSE_GO_CODEX_AUTH_FILE"
	authDirName     = ".goose-go"
	authFileName    = "auth.json"
	defaultTokenURL = "https://auth.openai.com/oauth/token"
	authClaimKey    = "https://api.openai.com/auth"
	defaultAuthMode = "chatgpt"
	refreshSkew     = 5 * time.Minute
	defaultFilePerm = 0o600
	defaultDirPerm  = 0o700
	defaultLockTTL  = 30 * time.Second
	defaultLockPoll = 100 * time.Millisecond
)

type Reader struct {
	authPath string
	tokenURL string
	client   *http.Client
	now      func() time.Time
	locker   lockManager
}

type Option func(*Reader)

type Credentials struct {
	AccessToken  string
	RefreshToken string
	AccountID    string
	ClientID     string
	ExpiresAt    time.Time
	LastRefresh  time.Time
	AuthMode     string
}

type tokenClaims struct {
	Exp      int64  `json:"exp"`
	ClientID string `json:"client_id"`
	Auth     struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"https://api.openai.com/auth"`
}

type authFile struct {
	AuthMode    string     `json:"auth_mode,omitempty"`
	LastRefresh string     `json:"last_refresh,omitempty"`
	Tokens      authTokens `json:"tokens"`
	extra       fieldValues
}

type authTokens struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
	extra        fieldValues
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type fieldValues map[string]json.RawMessage

type lockManager interface {
	Lock(ctx context.Context, path string) (func(), error)
}

type fileLockManager struct {
	now        func() time.Time
	sleep      func(time.Duration)
	staleAfter time.Duration
	poll       time.Duration
}

func NewReader(opts ...Option) (*Reader, error) {
	path, err := defaultAuthPath()
	if err != nil {
		return nil, err
	}

	reader := &Reader{
		authPath: path,
		tokenURL: defaultTokenURL,
		client:   http.DefaultClient,
		now:      func() time.Time { return time.Now().UTC() },
		locker: fileLockManager{
			now:        func() time.Time { return time.Now().UTC() },
			sleep:      time.Sleep,
			staleAfter: defaultLockTTL,
			poll:       defaultLockPoll,
		},
	}

	for _, opt := range opts {
		opt(reader)
	}

	return reader, nil
}

func WithAuthPath(path string) Option {
	return func(r *Reader) {
		r.authPath = path
	}
}

func WithTokenURL(rawURL string) Option {
	return func(r *Reader) {
		r.tokenURL = rawURL
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(r *Reader) {
		r.client = client
	}
}

func WithNow(now func() time.Time) Option {
	return func(r *Reader) {
		r.now = now
		if locker, ok := r.locker.(fileLockManager); ok {
			locker.now = now
			r.locker = locker
		}
	}
}

func WithLockManager(locker lockManager) Option {
	return func(r *Reader) {
		r.locker = locker
	}
}

func (r *Reader) Resolve(ctx context.Context) (Credentials, error) {
	file, err := r.load()
	if err != nil {
		return Credentials{}, err
	}

	creds, err := r.credentialsFromFile(file)
	if err != nil {
		return Credentials{}, err
	}

	if !r.needsRefresh(creds.ExpiresAt) {
		return creds, nil
	}

	if r.locker == nil {
		return Credentials{}, errors.New("codex auth lock manager is required")
	}
	unlock, err := r.locker.Lock(ctx, r.authPath+".lock")
	if err != nil {
		return Credentials{}, fmt.Errorf("lock codex auth file: %w", err)
	}
	defer unlock()

	file, err = r.load()
	if err != nil {
		return Credentials{}, err
	}
	creds, err = r.credentialsFromFile(file)
	if err != nil {
		return Credentials{}, err
	}
	if !r.needsRefresh(creds.ExpiresAt) {
		return creds, nil
	}

	updated, err := r.refresh(ctx, file, creds)
	if err != nil {
		reloaded, reloadErr := r.load()
		if reloadErr == nil {
			reloadedCreds, credsErr := r.credentialsFromFile(reloaded)
			if credsErr == nil && !r.needsRefresh(reloadedCreds.ExpiresAt) {
				return reloadedCreds, nil
			}
		}
		return Credentials{}, err
	}

	return r.credentialsFromFile(updated)
}

func defaultAuthPath() (string, error) {
	if override := os.Getenv(AuthFileEnvVar); override != "" {
		return override, nil
	}

	nativePath, err := nativeAuthPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(nativePath); err == nil {
		return nativePath, nil
	}

	legacyPath, err := legacyAuthPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, nil
	}

	return nativePath, nil
}

func nativeAuthPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(homeDir, authDirName, authFileName), nil
}

func legacyAuthPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(homeDir, ".codex", authFileName), nil
}

func writableAuthPath() (string, error) {
	if override := os.Getenv(AuthFileEnvVar); override != "" {
		return override, nil
	}
	return nativeAuthPath()
}

func loginHint() string {
	return "run `goose-go login` or `codex login`"
}

func (r *Reader) load() (authFile, error) {
	data, err := os.ReadFile(r.authPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return authFile{}, fmt.Errorf("codex auth file not found at %s; %s", r.authPath, loginHint())
		}
		return authFile{}, fmt.Errorf("read codex auth file: %w", err)
	}

	var file authFile
	if err := json.Unmarshal(data, &file); err != nil {
		return authFile{}, fmt.Errorf("decode codex auth file: %w", err)
	}

	return file, nil
}

func (r *Reader) credentialsFromFile(file authFile) (Credentials, error) {
	if file.AuthMode != "" && file.AuthMode != defaultAuthMode {
		return Credentials{}, fmt.Errorf("codex auth mode %q is not supported; %s", file.AuthMode, loginHint())
	}

	if file.Tokens.AccessToken == "" || file.Tokens.RefreshToken == "" {
		return Credentials{}, fmt.Errorf("codex auth file is missing access or refresh token; %s", loginHint())
	}

	accessClaims, err := parseTokenClaims(file.Tokens.AccessToken)
	if err != nil {
		return Credentials{}, fmt.Errorf("parse access token: %w", err)
	}

	accountID := file.Tokens.AccountID
	if accountID == "" {
		accountID = accessClaims.Auth.ChatGPTAccountID
	}
	if accountID == "" && file.Tokens.IDToken != "" {
		idClaims, err := parseTokenClaims(file.Tokens.IDToken)
		if err == nil {
			accountID = idClaims.Auth.ChatGPTAccountID
		}
	}
	if accountID == "" {
		return Credentials{}, fmt.Errorf("codex auth file is missing account_id and access token claim; %s", loginHint())
	}

	lastRefresh, err := parseLastRefresh(file.LastRefresh)
	if err != nil {
		return Credentials{}, fmt.Errorf("parse last_refresh: %w", err)
	}

	if accessClaims.Exp == 0 {
		return Credentials{}, errors.New("codex access token is missing exp claim")
	}

	return Credentials{
		AccessToken:  file.Tokens.AccessToken,
		RefreshToken: file.Tokens.RefreshToken,
		AccountID:    accountID,
		ClientID:     accessClaims.ClientID,
		ExpiresAt:    time.Unix(accessClaims.Exp, 0).UTC(),
		LastRefresh:  lastRefresh,
		AuthMode:     defaultAuthMode,
	}, nil
}

func parseLastRefresh(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func (r *Reader) needsRefresh(expiresAt time.Time) bool {
	return !expiresAt.After(r.now().Add(refreshSkew))
}

func (r *Reader) refresh(ctx context.Context, file authFile, creds Credentials) (authFile, error) {
	if creds.ClientID == "" {
		return authFile{}, fmt.Errorf("codex access token is missing client_id claim; %s", loginHint())
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", creds.RefreshToken)
	form.Set("client_id", creds.ClientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.tokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return authFile{}, fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.client.Do(req)
	if err != nil {
		return authFile{}, fmt.Errorf("refresh codex token: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return authFile{}, fmt.Errorf("refresh codex token: unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return authFile{}, fmt.Errorf("decode refresh response: %w", err)
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" || tokenResp.ExpiresIn <= 0 {
		return authFile{}, errors.New("refresh codex token: response missing required fields")
	}

	file.Tokens.AccessToken = tokenResp.AccessToken
	file.Tokens.RefreshToken = tokenResp.RefreshToken
	if tokenResp.IDToken != "" {
		file.Tokens.IDToken = tokenResp.IDToken
	}

	claims, err := parseTokenClaims(file.Tokens.AccessToken)
	if err != nil {
		return authFile{}, fmt.Errorf("parse refreshed access token: %w", err)
	}
	if claims.Auth.ChatGPTAccountID != "" {
		file.Tokens.AccountID = claims.Auth.ChatGPTAccountID
	}
	file.LastRefresh = r.now().Format(time.RFC3339)

	if err := writeAuthFileAtomically(r.authPath, file); err != nil {
		return authFile{}, err
	}

	return file, nil
}

func parseTokenClaims(token string) (tokenClaims, error) {
	parts := bytes.Split([]byte(token), []byte("."))
	if len(parts) != 3 {
		return tokenClaims{}, errors.New("invalid JWT format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(string(parts[1]))
	if err != nil {
		return tokenClaims{}, fmt.Errorf("decode JWT payload: %w", err)
	}

	var claims tokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return tokenClaims{}, fmt.Errorf("unmarshal JWT payload: %w", err)
	}

	return claims, nil
}

func writeAuthFileAtomically(path string, file authFile) error {
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode codex auth file: %w", err)
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), defaultDirPerm); err != nil {
		return fmt.Errorf("ensure auth dir: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".auth-*.json")
	if err != nil {
		return fmt.Errorf("create temp auth file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temp auth file: %w", err)
	}
	if err := tempFile.Chmod(defaultFilePerm); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("chmod temp auth file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp auth file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace auth file: %w", err)
	}

	return nil
}

func (f *authFile) UnmarshalJSON(data []byte) error {
	type alias authFile
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	delete(raw, "auth_mode")
	delete(raw, "last_refresh")
	delete(raw, "tokens")

	*f = authFile(parsed)
	f.extra = raw
	return nil
}

func (f authFile) MarshalJSON() ([]byte, error) {
	out := map[string]json.RawMessage{}
	for key, value := range f.extra {
		out[key] = value
	}

	if f.AuthMode != "" {
		value, err := json.Marshal(f.AuthMode)
		if err != nil {
			return nil, err
		}
		out["auth_mode"] = value
	}
	if f.LastRefresh != "" {
		value, err := json.Marshal(f.LastRefresh)
		if err != nil {
			return nil, err
		}
		out["last_refresh"] = value
	}
	value, err := json.Marshal(f.Tokens)
	if err != nil {
		return nil, err
	}
	out["tokens"] = value

	return json.Marshal(out)
}

func (m fileLockManager) Lock(ctx context.Context, path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), defaultDirPerm); err != nil {
		return nil, fmt.Errorf("ensure auth lock dir: %w", err)
	}
	for {
		lockFile, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, defaultFilePerm)
		if err == nil {
			_ = lockFile.Close()
			return func() {
				_ = os.Remove(path)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if m.isStale(path) {
			_ = os.Remove(path)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		m.sleep(m.poll)
	}
}

func (m fileLockManager) isStale(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return m.now().Sub(info.ModTime()) > m.staleAfter
}

func (t *authTokens) UnmarshalJSON(data []byte) error {
	type alias authTokens
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	var parsed alias
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}

	delete(raw, "access_token")
	delete(raw, "refresh_token")
	delete(raw, "id_token")
	delete(raw, "account_id")

	*t = authTokens(parsed)
	t.extra = raw
	return nil
}

func (t authTokens) MarshalJSON() ([]byte, error) {
	out := map[string]json.RawMessage{}
	for key, value := range t.extra {
		out[key] = value
	}

	fields := map[string]string{
		"access_token":  t.AccessToken,
		"refresh_token": t.RefreshToken,
		"id_token":      t.IDToken,
		"account_id":    t.AccountID,
	}
	for key, raw := range fields {
		if raw == "" {
			continue
		}
		value, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		out[key] = value
	}

	return json.Marshal(out)
}
