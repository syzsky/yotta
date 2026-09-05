package nativeoidc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/yottaapp/yotta/internal/securestore"
)

// Only the refresh token crosses the persistence seam. Access tokens, account
// passwords and browser cookies never enter ordinary files or frontend state.
func (s *Session) credentialKey() string {
	sum := sha256.Sum256([]byte(s.config.CredentialScope + "\x00" + s.config.TokenEndpoint + "\x00" + s.config.ClientID + "\x00" + s.config.Audience))
	return "Yotta/account/" + hex.EncodeToString(sum[:])
}
func (s *Session) sessionOnly(value bool) {
	s.stateMu.Lock()
	s.profile.SessionOnly = value
	s.stateMu.Unlock()
}
func (s *Session) restoreLocked() {
	if s.credentialsLoaded {
		return
	}
	s.credentialsLoaded = true
	if s.config.Credentials == nil {
		return
	}
	token, err := s.config.Credentials.Get(s.credentialKey())
	if errors.Is(err, securestore.ErrNotFound) {
		return
	}
	if err != nil {
		s.sessionOnly(true)
		s.credentialsLoaded = false
		return
	}
	if strings.TrimSpace(token) == "" || len(token) > 16384 {
		s.sessionOnly(true)
		return
	}
	s.refreshToken = token
}
func (s *Session) persistLocked() {
	s.credentialsLoaded = true
	if s.config.Credentials == nil || s.refreshToken == "" {
		s.sessionOnly(true)
		return
	}
	s.sessionOnly(s.config.Credentials.Set(s.credentialKey(), s.refreshToken) != nil)
}

func (s *Session) RefreshProfile(ctx context.Context) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.tokenLocked(ctx, false)
	if errors.Is(err, ErrAuthenticationRequired) {
		return s.Profile(), nil
	}
	if err != nil {
		return s.Profile(), err
	}
	if err := s.fetchProfileLocked(ctx, token); err != nil {
		return s.Profile(), err
	}
	return s.Profile(), nil
}
func (s *Session) fetchProfileLocked(ctx context.Context, token string) error {
	if s.config.UserinfoEndpoint == "" {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.UserinfoEndpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == 401 {
		s.token, s.refreshToken = "", ""
		if s.config.Credentials != nil {
			_ = s.config.Credentials.Delete(s.credentialKey())
		}
		s.stateMu.Lock()
		s.profile = Profile{}
		s.stateMu.Unlock()
		return ErrAuthenticationRequired
	}
	var p Profile
	if response.StatusCode != 200 || json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&p) != nil || p.UserKey == "" {
		return errors.New("native OIDC profile retrieval failed")
	}
	s.stateMu.Lock()
	p.SessionOnly = s.profile.SessionOnly
	s.profile = p
	s.stateMu.Unlock()
	return nil
}
func (s *Session) OpenAccountCenter() error {
	if s.config.AccountURL == "" {
		return errors.New("account center URL not configured")
	}
	return s.config.Browser.OpenURL(s.config.AccountURL)
}
