package workflow

import (
	"context"
	"github.com/yottaapp/yotta/internal/nativeoidc"
)

func (s *Service) RegistryAccount() nativeoidc.Profile {
	if s.account == nil {
		return nativeoidc.Profile{}
	}
	return s.account.Profile()
}

func (s *Service) LoginRegistry(ctx context.Context) (nativeoidc.Profile, error) {
	if s.account == nil {
		return nativeoidc.Profile{}, unavailable("registry")
	}
	if _, err := s.account.Token(ctx); err != nil {
		return nativeoidc.Profile{}, registryError("login", err)
	}
	return s.account.Profile(), nil
}

func (s *Service) CancelRegistryLogin() {
	if s.account != nil {
		s.account.CancelLogin()
	}
}
func (s *Service) LogoutRegistry() {
	if s.account != nil {
		s.account.Logout()
	}
}
