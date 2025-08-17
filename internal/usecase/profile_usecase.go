// Package usecase contains the application-specific business rules.
package usecase

import (
	"context"

	"radar/internal/domain/entity"

	"github.com/google/uuid"
)

// ProfileUsecase defines the interface for profile-related business operations.
type ProfileUsecase interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (*entity.User, error)
	UpdateUserProfile(ctx context.Context, userID uuid.UUID, input *UpdateUserProfileInput) error
	UpdateMerchantProfile(ctx context.Context, userID uuid.UUID, input *UpdateMerchantProfileInput) error
	SwitchToMerchant(ctx context.Context, userID uuid.UUID, input *SwitchToMerchantInput) error
	GetUserRole(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// --- Input DTOs ---

// UpdateUserProfileInput defines the data required to update a user profile.
type UpdateUserProfileInput struct {
	LoyaltyPoints *int `json:"loyalty_points,omitempty"`
}

// UpdateMerchantProfileInput defines the data required to update a merchant profile.
type UpdateMerchantProfileInput struct {
	StoreName        *string `json:"store_name,omitempty"`
	StoreDescription *string `json:"store_description,omitempty"`
	BusinessLicense  *string `json:"business_license,omitempty"`
}

// SwitchToMerchantInput defines the data required to switch a user to merchant role.
type SwitchToMerchantInput struct {
	StoreName       string `json:"store_name"`
	BusinessLicense string `json:"business_license"`
}
