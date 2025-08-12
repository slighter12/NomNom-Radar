// Package usecase contains the application-specific business rules.
// It orchestrates the domain layer to perform tasks.
package usecase

import (
	"context"

	"radar/internal/domain/entity"
)

// --- Input DTOs ---

// RegisterUserInput defines the data required to register a new user.
type RegisterUserInput struct {
	Name     string
	Email    string
	Password string
}

// RegisterMerchantInput defines the data required to register a new merchant.
type RegisterMerchantInput struct {
	Name            string
	Email           string
	Password        string
	StoreName       string
	BusinessLicense string
}

// LoginInput defines the data required for a user to log in.
type LoginInput struct {
	Email    string
	Password string
}

// RefreshTokenInput defines the data required to refresh an access token.
type RefreshTokenInput struct {
	RefreshToken string
}

// LogoutInput defines the data required to log out.
type LogoutInput struct {
	RefreshToken string
}

// GoogleCallbackInput defines the data required for Google login.
type GoogleCallbackInput struct {
	IDToken string `json:"id_token"`
}

// --- Output DTOs ---

// RegisterOutput returns the newly created user's basic information.
type RegisterOutput struct {
	User *entity.User
}

// LoginOutput returns the generated tokens after a successful login.
type LoginOutput struct {
	AccessToken  string
	RefreshToken string
	User         *entity.User
}

// RefreshTokenOutput returns the new generated tokens.
type RefreshTokenOutput struct {
	AccessToken  string
	RefreshToken string
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
}
