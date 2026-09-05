package nativeoidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/yottaapp/yotta/internal/securestore"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Browser interface{ OpenURL(string) error }

var ErrAuthenticationRequired = errors.New("native session requires explicit sign-in")
var ErrCredentialStorage = errors.New("saved account could not be removed")

type Config struct {
	Credentials           securestore.Store
	CredentialScope       string
	AccountURL            string
	AuthorizationEndpoint string
	TokenEndpoint         string
	ClientID              string
	Audience              string
	Scopes                []string
	CallbackAddress       string
	Browser               Browser
	HTTPClient            *http.Client
	LoginTimeout          time.Duration
	UserinfoEndpoint      string
}

type Session struct {
	credentialsLoaded bool
	config            Config
	client            *http.Client
	mu                sync.Mutex
	token             string
	refreshToken      string
	expiresAt         time.Time
	stateMu           sync.Mutex
	cancel            context.CancelFunc
	profile           Profile
}

type Profile struct {
	SessionOnly bool   `json:"sessionOnly"`
	UserKey     string `json:"user_key"`
	Name        string `json:"name"`
	Picture     string `json:"picture"`
	SigningIn   bool   `json:"signingIn"`
}

func (session *Session) Profile() Profile {
	session.stateMu.Lock()
	defer session.stateMu.Unlock()
	p := session.profile
	p.SigningIn = session.cancel != nil
	return p
}

func (session *Session) CancelLogin() {
	session.stateMu.Lock()
	defer session.stateMu.Unlock()
	if session.cancel != nil {
		session.cancel()
	}
}

func (session *Session) Logout() error {
	session.CancelLogin()
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.config.Credentials != nil {
		if err := session.config.Credentials.Delete(session.credentialKey()); err != nil && !errors.Is(err, securestore.ErrNotFound) && !errors.Is(err, securestore.ErrUnavailable) {
			return errors.Join(ErrCredentialStorage, err)
		}
	}
	session.credentialsLoaded = true
	session.token, session.refreshToken = "", ""
	session.stateMu.Lock()
	session.profile = Profile{}
	session.stateMu.Unlock()
	return nil
}

func New(config Config) (*Session, error) {
	if config.AccountURL != "" {
		u, err := url.Parse(config.AccountURL)
		if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback()))) {
			return nil, errors.New("invalid account center URL")
		}
	}
	for _, endpoint := range []string{config.AuthorizationEndpoint, config.TokenEndpoint} {
		parsed, err := url.Parse(endpoint)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" {
			return nil, errors.New("native OIDC endpoints must be absolute URLs")
		}
	}
	host, _, err := net.SplitHostPort(config.CallbackAddress)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return nil, errors.New("native OIDC callback must use a fixed loopback address")
	}
	if strings.TrimSpace(config.ClientID) == "" || config.Browser == nil {
		return nil, errors.New("native OIDC client ID and browser are required")
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if config.LoginTimeout <= 0 {
		config.LoginTimeout = 2 * time.Minute
	}
	return &Session{config: config, client: client}, nil
}

func (session *Session) Token(ctx context.Context) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.tokenLocked(ctx, false)
}

// Only explicit login actions may open the system browser. API calls can
// refresh credentials silently, but must return an authentication problem.
func (session *Session) Login(ctx context.Context) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, session.config.LoginTimeout)
	session.stateMu.Lock()
	session.cancel = cancel
	session.stateMu.Unlock()
	defer func() { cancel(); session.stateMu.Lock(); session.cancel = nil; session.stateMu.Unlock() }()
	token, err := session.tokenLocked(ctx, true)
	if err != nil {
		return "", err
	}
	if err := session.fetchProfileLocked(ctx, token); err != nil {
		return "", err
	}
	return token, nil
}

func (session *Session) tokenLocked(ctx context.Context, interactive bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	session.restoreLocked()
	if session.token != "" && time.Until(session.expiresAt) > time.Minute {
		return session.token, nil
	}
	if session.refreshToken != "" {
		if token, err := session.refresh(ctx); err == nil {
			return token, nil
		} else if !errors.Is(err, ErrAuthenticationRequired) {
			return "", err // Offline/5xx/cancellation must not erase remembered login.
		}
		if session.config.Credentials != nil {
			_ = session.config.Credentials.Delete(session.credentialKey())
		}
		session.token = ""
		session.refreshToken = ""
		session.stateMu.Lock()
		session.profile = Profile{}
		session.stateMu.Unlock()
	}
	if !interactive {
		return "", ErrAuthenticationRequired
	}
	return session.login(ctx)
}

func (session *Session) login(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	verifier, err := randomURLToken(48)
	if err != nil {
		return "", err
	}
	state, err := randomURLToken(32)
	if err != nil {
		return "", err
	}
	challengeBytes := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(challengeBytes[:])
	listener, err := net.Listen("tcp", session.config.CallbackAddress)
	if err != nil {
		return "", fmt.Errorf("open OIDC callback: %w", err)
	}
	defer listener.Close()
	redirectURI := "http://" + session.config.CallbackAddress + "/callback"
	result := make(chan callbackResult, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/callback" || request.URL.Query().Get("state") != state {
			http.Error(response, "登录请求无效", http.StatusBadRequest)
			return
		}
		select {
		case result <- callbackResult{code: request.URL.Query().Get("code"), oauthError: request.URL.Query().Get("error")}:
		default:
			http.Error(response, "登录请求已经处理", http.StatusConflict)
			return
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = response.Write([]byte("<!doctype html><meta charset=utf-8><title>Yotta</title><p>登录完成，可以返回 Yotta。</p>"))
	})
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	authorizationURL, _ := url.Parse(session.config.AuthorizationEndpoint)
	query := authorizationURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", session.config.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(session.config.Scopes, " "))
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	if session.config.Audience != "" {
		query.Set("audience", session.config.Audience)
	}
	authorizationURL.RawQuery = query.Encode()
	if err := session.config.Browser.OpenURL(authorizationURL.String()); err != nil {
		return "", err
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case callback := <-result:
		if callback.oauthError != "" || callback.code == "" {
			return "", errors.New("native OIDC authorization was not completed")
		}
		token, err := session.exchange(ctx, callback.code, verifier, redirectURI)
		if err != nil {
			return "", err
		}
		return token, nil
	}
}

func (session *Session) exchange(ctx context.Context, code, verifier, redirectURI string) (string, error) {
	form := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {session.config.ClientID},
		"code": {code}, "code_verifier": {verifier}, "redirect_uri": {redirectURI},
	}
	return session.requestToken(ctx, form)
}

func (session *Session) refresh(ctx context.Context) (string, error) {
	return session.requestToken(ctx, url.Values{
		"grant_type": {"refresh_token"}, "client_id": {session.config.ClientID},
		"refresh_token": {session.refreshToken},
	})
}

func (session *Session) requestToken(ctx context.Context, form url.Values) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, session.config.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := session.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var problem struct {
			Code string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&problem)
		if (response.StatusCode == 400 || response.StatusCode == 401) && (problem.Code == "invalid_grant" || problem.Code == "invalid_token") {
			return "", ErrAuthenticationRequired
		}
		return "", errors.New("native OIDC token service unavailable")
	}
	var token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&token) != nil || token.AccessToken == "" || token.ExpiresIn <= 0 {
		return "", errors.New("native OIDC token exchange failed")
	}
	session.token = token.AccessToken
	if token.RefreshToken != "" {
		session.refreshToken = token.RefreshToken
	}
	session.expiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	session.persistLocked()
	return session.token, nil
}

type callbackResult struct{ code, oauthError string }

func randomURLToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
