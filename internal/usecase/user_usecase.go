// Package usecase contains the application-specific business rules.
// It orchestrates the domain layer to perform tasks.
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshTokenInput defines the data required to refresh an access token.
type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutInput defines the data required to log out.
type LogoutInput struct {
	RefreshToken string `json:"refresh_token"`
}

// GoogleCallbackInput defines the data required for Google login.
type GoogleCallbackInput struct {
	IDToken string `json:"id_token"`
	State   string `json:"state,omitempty"` // State parameter for CSRF protection
}

// --- Output DTOs ---

// RegisterOutput returns the newly created user's basic information.
type RegisterOutput struct {
	User *entity.User `json:"user"`
}

// LoginOutput returns the generated tokens after a successful login.
type LoginOutput struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User         *entity.User `json:"user"`
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
	RegisterUser(ctx context.Context, input *RegisterUserInput) (*RegisterOutput, error)
	RegisterMerchant(ctx context.Context, input *RegisterMerchantInput) (*RegisterOutput, error)
	Login(ctx context.Context, input *LoginInput) (*LoginOutput, error)
	RefreshToken(ctx context.Context, input *RefreshTokenInput) (*RefreshTokenOutput, error)
	Logout(ctx context.Context, input *LogoutInput) error
	GoogleCallback(ctx context.Context, input *GoogleCallbackInput) (*LoginOutput, error)

	// Session management methods
	LogoutAllDevices(ctx context.Context, userID uuid.UUID) error
	GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error)
	RevokeSession(ctx context.Context, userID, tokenID uuid.UUID) error

	// Google OAuth account management
	LinkGoogleAccount(ctx context.Context, userID uuid.UUID, idToken string) error
	UnlinkGoogleAccount(ctx context.Context, userID uuid.UUID) error
}
