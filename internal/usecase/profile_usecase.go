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
	GetMerchantDiscoveryProfile(ctx context.Context, userID uuid.UUID) (*MerchantDiscoveryProfileResult, error)
	UpdateMerchantDiscoveryProfile(ctx context.Context, userID uuid.UUID, input *UpdateMerchantDiscoveryProfileInput) (*MerchantDiscoveryProfileResult, error)
	SubmitMerchantVerification(ctx context.Context, userID uuid.UUID, input *SubmitMerchantVerificationInput) error
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
}

type OptionalUUIDUpdate struct {
	IsSet bool
	Value *uuid.UUID
}

type UpdateMerchantDiscoveryProfileInput struct {
	DiscoveryCategoryID    OptionalUUIDUpdate
	DiscoverySubcategoryID OptionalUUIDUpdate
	ActiveHubID            OptionalUUIDUpdate
	IsPublic               *bool
}

type MerchantDiscoveryProfileResult struct {
	DiscoveryCategoryID      *uuid.UUID                   `json:"discovery_category_id,omitempty"`
	DiscoverySubcategoryID   *uuid.UUID                   `json:"discovery_subcategory_id,omitempty"`
	ActiveHubID              *uuid.UUID                   `json:"active_hub_id,omitempty"`
	IsPublic                 bool                         `json:"is_public"`
	IsVerified               bool                         `json:"is_verified"`
	HasActivePrimaryLocation bool                         `json:"has_active_primary_location"`
	DiscoveryCategory        *entity.DiscoveryCategory    `json:"discovery_category,omitempty"`
	DiscoverySubcategory     *entity.DiscoverySubcategory `json:"discovery_subcategory,omitempty"`
	ActiveHub                *entity.Hub                  `json:"active_hub,omitempty"`
}

type SubmitMerchantVerificationInput struct {
	BusinessLicense string `json:"business_license" validate:"required"`
}

// SwitchToMerchantInput defines the data required to switch a user to merchant role.
type SwitchToMerchantInput struct {
	StoreName string `json:"store_name"`
}
