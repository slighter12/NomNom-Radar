package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// --- Input DTOs ---

// RegisterUserInput defines the data required to register a new user.
type RegisterUserInput struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RegisterMerchantInput defines the data required to register a new merchant.
type RegisterMerchantInput struct {
	Name            string `json:"name" validate:"required"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required"`
	StoreName       string `json:"store_name" validate:"required"`
	BusinessLicense string `json:"business_license" validate:"required"`
}

// LoginInput defines the data required for a user to log in.
type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshTokenInput defines the data required to refresh an access token.
type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// LogoutInput defines the data required to log out.
type LogoutInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// GoogleCallbackInput defines the data required for Google login.
type GoogleCallbackInput struct {
	IDToken         string `json:"id_token" validate:"required"`
	RequestedRole   string `json:"requested_role,omitempty" validate:"omitempty,oneof=user merchant"`
	State           string `json:"state,omitempty" validate:"omitempty,oneof=user merchant"` // Deprecated: use requested_role instead.
	StoreName       string `json:"store_name,omitempty"`
	BusinessLicense string `json:"business_license,omitempty"`
}

// CompleteMerchantOnboardingInput defines the data required to finish merchant onboarding.
type CompleteMerchantOnboardingInput struct {
	OnboardingToken string `json:"onboarding_token" validate:"required"`
	StoreName       string `json:"store_name" validate:"required"`
	BusinessLicense string `json:"business_license" validate:"required"`
}

// --- Output DTOs ---

type AuthStatus string

const (
	AuthStatusAuthenticated      AuthStatus = "authenticated"
	AuthStatusOnboardingRequired AuthStatus = "onboarding_required"
)

// AuthResult returns the result of an authentication attempt.
type AuthResult struct {
	Status          AuthStatus   `json:"status"`
	AccessToken     string       `json:"access_token,omitempty"`
	RefreshToken    string       `json:"refresh_token,omitempty"`
	OnboardingToken string       `json:"onboarding_token,omitempty"`
	RequestedRole   string       `json:"requested_role,omitempty"`
	RequiredFields  []string     `json:"required_fields,omitempty"`
	User            *entity.User `json:"user,omitempty"`
}

// RefreshTokenOutput returns the new generated access token only.
// The refresh token remains unchanged for security reasons.
type RefreshTokenOutput struct {
	AccessToken string `json:"access_token"`
	// Note: Refresh token is not returned to maintain security
	// and prevent token rotation attacks
}

// UserUsecase defines the interface for user-related business operations.
// This is the contract that the delivery layer (e.g., API handlers) will depend on.
type UserUsecase interface {
	RegisterUser(ctx context.Context, input *RegisterUserInput) (*AuthResult, error)
	RegisterMerchant(ctx context.Context, input *RegisterMerchantInput) (*AuthResult, error)
	Login(ctx context.Context, input *LoginInput) (*AuthResult, error)
	RefreshToken(ctx context.Context, input *RefreshTokenInput) (*RefreshTokenOutput, error)
	Logout(ctx context.Context, input *LogoutInput) error
	GoogleCallback(ctx context.Context, input *GoogleCallbackInput) (*AuthResult, error)
	CompleteMerchantOnboarding(ctx context.Context, input *CompleteMerchantOnboardingInput) (*AuthResult, error)

	// Session management methods
	LogoutAllDevices(ctx context.Context, userID uuid.UUID) error
	GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error)
	RevokeSession(ctx context.Context, userID, tokenID uuid.UUID) error

	// Google OAuth account management
	LinkGoogleAccount(ctx context.Context, userID uuid.UUID, idToken string) error
	UnlinkGoogleAccount(ctx context.Context, userID uuid.UUID) error
}
