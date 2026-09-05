package workflow

import (
	"context"
	"errors"
	"github.com/yottaapp/yotta/internal/apperr"
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
	if _, err := s.account.Login(ctx); err != nil {
		return nativeoidc.Profile{}, registryError("login", err)
	}
	return s.account.Profile(), nil
}

func (s *Service) CancelRegistryLogin() {
	if s.account != nil {
		s.account.CancelLogin()
	}
}
func (s *Service) LogoutRegistry() error {
	if s.account != nil {
		return accountError(s.account.Logout())
	}
	return nil
}

func (s *Service) RefreshRegistryAccount(ctx context.Context) (nativeoidc.Profile, error) {
	if s.account == nil {
		return nativeoidc.Profile{}, nil
	}
	profile, err := s.account.RefreshProfile(ctx)
	return profile, accountError(err)
}
func (s *Service) OpenAccountCenter() error {
	if s.account == nil {
		return unavailable("account")
	}
	return accountError(s.account.OpenAccountCenter())
}
func accountError(cause error) error {
	if cause == nil {
		return nil
	}
	if errors.Is(cause, nativeoidc.ErrCredentialStorage) {
		return projectError("workflow.account.sign_out_failed", apperr.CategoryInfrastructure, nil, true, cause)
	}
	if errors.Is(cause, nativeoidc.ErrAuthenticationRequired) || errors.Is(cause, context.Canceled) {
		return registryError("account", cause)
	}
	return projectError("workflow.account.unavailable", apperr.CategoryInfrastructure, nil, true, cause)
}
