package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/gosuda/aira/internal/auth"
	"github.com/gosuda/aira/internal/domain"
)

type RegisterInput struct {
	Body struct {
		TenantSlug string `json:"tenant_slug" minLength:"1" maxLength:"63" doc:"Tenant slug"`
		Email      string `json:"email" minLength:"3" maxLength:"255" doc:"User email"`
		Password   string `json:"password" minLength:"8" maxLength:"128" doc:"Password"` //nolint:gosec // G117: login credential DTO
		Name       string `json:"name" minLength:"1" maxLength:"255" doc:"Display name"`
	}
}

type RegisterOutput struct {
	Body struct {
		User         *domain.User `json:"user"`
		AccessToken  string       `json:"access_token"`  //nolint:gosec // G117: auth response DTO
		RefreshToken string       `json:"refresh_token"` //nolint:gosec // G117: auth response DTO
	}
}

type LoginInput struct {
	Body struct {
		TenantSlug string `json:"tenant_slug" minLength:"1" maxLength:"63" doc:"Tenant slug"`
		Email      string `json:"email" minLength:"3" maxLength:"255" doc:"User email"`
		Password   string `json:"password" minLength:"1" maxLength:"128" doc:"Password"` //nolint:gosec // G117: login credential DTO
	}
}

type LoginOutput struct {
	Body struct {
		AccessToken  string `json:"access_token"`  //nolint:gosec // G117: auth response DTO
		RefreshToken string `json:"refresh_token"` //nolint:gosec // G117: auth response DTO
	}
}

type RefreshInput struct {
	Body struct {
		RefreshToken string `json:"refresh_token" minLength:"1" doc:"Refresh token"` //nolint:gosec // G117: token refresh DTO
	}
}

type RefreshOutput struct {
	Body struct {
		AccessToken string `json:"access_token"` //nolint:gosec // G117: auth response DTO
	}
}

func RegisterAuthRoutes(api huma.API, store DataStore, authSvc AuthService) {
	huma.Register(api, huma.Operation{
		OperationID: "register",
		Method:      http.MethodPost,
		Path:        "/auth/register",
		Summary:     "Register a new user",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *RegisterInput) (*RegisterOutput, error) {
		tenant, err := store.Tenants().GetBySlug(ctx, input.Body.TenantSlug)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("tenant not found")
			}
			return nil, huma.Error500InternalServerError("failed to look up tenant", err)
		}

		user, err := authSvc.Register(ctx, tenant.ID, input.Body.Email, input.Body.Password, input.Body.Name)
		if err != nil {
			if errors.Is(err, auth.ErrUserAlreadyExists) {
				return nil, huma.Error409Conflict("user already exists")
			}
			return nil, huma.Error500InternalServerError("failed to register user", err)
		}

		accessToken, refreshToken, err := authSvc.Login(ctx, tenant.ID, input.Body.Email, input.Body.Password)
		if err != nil {
			return nil, huma.Error500InternalServerError("registered but failed to issue tokens", err)
		}

		user.PasswordHash = ""

		out := &RegisterOutput{}
		out.Body.User = user
		out.Body.AccessToken = accessToken
		out.Body.RefreshToken = refreshToken
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/auth/login",
		Summary:     "Login with email and password",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
		tenant, err := store.Tenants().GetBySlug(ctx, input.Body.TenantSlug)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, huma.Error404NotFound("tenant not found")
			}
			return nil, huma.Error500InternalServerError("failed to look up tenant", err)
		}

		accessToken, refreshToken, err := authSvc.Login(ctx, tenant.ID, input.Body.Email, input.Body.Password)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidCredentials) {
				return nil, huma.Error401Unauthorized("invalid email or password")
			}
			return nil, huma.Error500InternalServerError("login failed", err)
		}

		out := &LoginOutput{}
		out.Body.AccessToken = accessToken
		out.Body.RefreshToken = refreshToken
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "refresh-token",
		Method:      http.MethodPost,
		Path:        "/auth/refresh",
		Summary:     "Refresh access token",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *RefreshInput) (*RefreshOutput, error) {
		accessToken, err := authSvc.RefreshToken(ctx, input.Body.RefreshToken)
		if err != nil {
			return nil, huma.Error401Unauthorized("invalid or expired refresh token")
		}

		out := &RefreshOutput{}
		out.Body.AccessToken = accessToken
		return out, nil
	})
}
