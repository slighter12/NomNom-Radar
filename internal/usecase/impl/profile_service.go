package impl

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/platform/observability"
	"radar/internal/usecase"

	"github.com/google/uuid"
)

// profileService implements the ProfileUsecase interface.
type profileService struct {
	txManager repository.TransactionManager
	logger    *slog.Logger
}

// NewProfileService is the constructor for profileService.
func NewProfileService(
	txManager repository.TransactionManager,
	logger *slog.Logger,
) usecase.ProfileUsecase {
	return &profileService{
		txManager: txManager,
		logger:    logger,
	}
}

// log returns a request-scoped logger if available, otherwise falls back to the service's logger.
func (srv *profileService) log(ctx context.Context) *slog.Logger {
	return observability.LoggerFromContextOrDefault(ctx, srv.logger)
}

// GetProfile retrieves the complete user profile including role-specific data.
func (srv *profileService) GetProfile(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	srv.log(ctx).Debug("Getting user profile", slog.String("user_id", userID.String()))

	var user *entity.User

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		foundUser, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrNotFound)
			}

			return err
		}
		user = foundUser

		return nil
	})

	if err != nil {
		return nil, err
	}

	return user, nil
}

// UpdateUserProfile updates the user profile data.
func (srv *profileService) UpdateUserProfile(ctx context.Context, userID uuid.UUID, input *usecase.UpdateUserProfileInput) error {
	srv.log(ctx).Info("Updating user profile", slog.String("user_id", userID.String()))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		// 1. Find the user
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrNotFound)
			}

			return err
		}

		// 2. Check if user has a user profile
		if user.UserProfile == nil {
			return domainerrors.ErrValidationFailed.WithDetails("user does not have a user profile")
		}

		// 3. Update the profile fields
		if input.LoyaltyPoints != nil {
			user.UserProfile.LoyaltyPoints = *input.LoyaltyPoints
		}

		// 4. Save the updated user
		if err := userRepo.Update(ctx, user); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// UpdateMerchantProfile updates the merchant profile data.
func (srv *profileService) UpdateMerchantProfile(ctx context.Context, userID uuid.UUID, input *usecase.UpdateMerchantProfileInput) error {
	srv.log(ctx).Info("Updating merchant profile", slog.String("user_id", userID.String()))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		// 1. Find the user
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrNotFound)
			}

			return err
		}

		// 2. Check if user has a merchant profile
		if user.MerchantProfile == nil {
			return domainerrors.ErrValidationFailed.WithDetails("user does not have a merchant profile")
		}

		// 3. Update the profile fields
		if input.StoreName != nil {
			user.MerchantProfile.StoreName = *input.StoreName
		}
		if input.StoreDescription != nil {
			user.MerchantProfile.StoreDescription = *input.StoreDescription
		}
		// 4. Save the updated user
		if err := userRepo.Update(ctx, user); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (srv *profileService) GetMerchantDiscoveryProfile(ctx context.Context, userID uuid.UUID) (*usecase.MerchantDiscoveryProfileResult, error) {
	srv.log(ctx).Debug("Getting merchant discovery profile", slog.String("user_id", userID.String()))

	var result *usecase.MerchantDiscoveryProfileResult

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		discoveryRepo := repoFactory.DiscoveryRepo()
		addressRepo := repoFactory.AddressRepo()

		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrNotFound)
			}

			return err
		}
		if user.MerchantProfile == nil {
			return domainerrors.ErrValidationFailed.WithDetails("user does not have a merchant profile")
		}

		hasActivePrimaryLocation, err := merchantHasActivePrimaryLocation(ctx, addressRepo, userID)
		if err != nil {
			return err
		}

		resolved, err := buildMerchantDiscoveryProfileResult(ctx, discoveryRepo, user.MerchantProfile, hasActivePrimaryLocation)
		if err != nil {
			return err
		}
		result = resolved

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (srv *profileService) UpdateMerchantDiscoveryProfile(ctx context.Context, userID uuid.UUID, input *usecase.UpdateMerchantDiscoveryProfileInput) (*usecase.MerchantDiscoveryProfileResult, error) {
	srv.log(ctx).Info("Updating merchant discovery profile", slog.String("user_id", userID.String()))
	if input == nil {
		return nil, domainerrors.ErrValidationFailed.WithDetails("discovery profile input is required")
	}

	var result *usecase.MerchantDiscoveryProfileResult

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		resolved, err := updateMerchantDiscoveryProfile(ctx, repoFactory, userID, input)
		if err != nil {
			return err
		}
		result = resolved

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

type merchantDiscoveryProfileUpdate struct {
	categoryID    *uuid.UUID
	subcategoryID *uuid.UUID
	hubID         *uuid.UUID
	isPublic      bool
}

func updateMerchantDiscoveryProfile(
	ctx context.Context,
	repoFactory repository.RepositoryFactory,
	userID uuid.UUID,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
) (*usecase.MerchantDiscoveryProfileResult, error) {
	userRepo := repoFactory.UserRepo()
	discoveryRepo := repoFactory.DiscoveryRepo()
	addressRepo := repoFactory.AddressRepo()

	user, profile, err := findMerchantProfileForUpdate(ctx, userRepo, userID)
	if err != nil {
		return nil, err
	}

	update := buildMerchantDiscoveryProfileUpdate(profile, input)
	if err := validateMerchantDiscoveryUpdateValues(ctx, discoveryRepo, input, update); err != nil {
		return nil, err
	}

	hasActivePrimaryLocation, err := merchantHasActivePrimaryLocation(ctx, addressRepo, userID)
	if err != nil {
		return nil, err
	}
	if err := validatePublicMerchantDiscoveryUpdate(profile, update, hasActivePrimaryLocation); err != nil {
		return nil, err
	}

	applyMerchantDiscoveryProfileUpdate(profile, update)
	if err := userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return buildMerchantDiscoveryProfileResult(ctx, discoveryRepo, profile, hasActivePrimaryLocation)
}

func findMerchantProfileForUpdate(
	ctx context.Context,
	userRepo repository.UserRepository,
	userID uuid.UUID,
) (*entity.User, *entity.MerchantProfile, error) {
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domainerrors.ErrUserNotFound) {
			return nil, nil, replaceWithSourceStack(err, domainerrors.ErrNotFound)
		}

		return nil, nil, err
	}
	if user.MerchantProfile == nil {
		return nil, nil, domainerrors.ErrValidationFailed.WithDetails("user does not have a merchant profile")
	}

	return user, user.MerchantProfile, nil
}

func buildMerchantDiscoveryProfileUpdate(
	profile *entity.MerchantProfile,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
) merchantDiscoveryProfileUpdate {
	update := merchantDiscoveryProfileUpdate{
		categoryID:    profile.DiscoveryCategoryID,
		subcategoryID: profile.DiscoverySubcategoryID,
		hubID:         profile.ActiveHubID,
		isPublic:      profile.IsPublic,
	}

	if input.DiscoveryCategoryID.IsSet {
		update.categoryID = input.DiscoveryCategoryID.Value
	}
	if input.DiscoverySubcategoryID.IsSet {
		update.subcategoryID = input.DiscoverySubcategoryID.Value
	}
	if input.ActiveHubID.IsSet {
		update.hubID = input.ActiveHubID.Value
	}
	if input.IsPublic != nil {
		update.isPublic = *input.IsPublic
	}

	return update
}

func validatePublicMerchantDiscoveryUpdate(
	profile *entity.MerchantProfile,
	update merchantDiscoveryProfileUpdate,
	hasActivePrimaryLocation bool,
) error {
	if !update.isPublic {
		return nil
	}

	return validateMerchantDiscoveryEligibility(profile, update.categoryID, update.subcategoryID, hasActivePrimaryLocation)
}

func applyMerchantDiscoveryProfileUpdate(
	profile *entity.MerchantProfile,
	update merchantDiscoveryProfileUpdate,
) {
	profile.DiscoveryCategoryID = update.categoryID
	profile.DiscoverySubcategoryID = update.subcategoryID
	profile.ActiveHubID = update.hubID
	profile.IsPublic = update.isPublic
}

func (srv *profileService) SubmitMerchantVerification(ctx context.Context, userID uuid.UUID, input *usecase.SubmitMerchantVerificationInput) error {
	srv.log(ctx).Info("Submitting merchant verification", slog.String("user_id", userID.String()))

	businessLicense := strings.TrimSpace(input.BusinessLicense)
	if businessLicense == "" {
		return domainerrors.ErrValidationFailed.WithDetails("business_license is required")
	}

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrNotFound)
			}

			return err
		}

		if user.MerchantProfile == nil {
			return domainerrors.ErrValidationFailed.WithDetails("user does not have a merchant profile")
		}

		currentBusinessLicense := strings.TrimSpace(user.MerchantProfile.BusinessLicense)
		if user.MerchantProfile.VerificationStatus == entity.MerchantVerificationStatusVerified {
			if currentBusinessLicense == businessLicense {
				return nil
			}

			return domainerrors.ErrConflict.WithDetails("merchant business license has already been verified")
		}

		now := time.Now()
		user.MerchantProfile.BusinessLicense = businessLicense
		user.MerchantProfile.VerificationStatus = entity.MerchantVerificationStatusVerified
		user.MerchantProfile.BusinessLicenseVerifiedAt = &now

		if err := userRepo.Update(ctx, user); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// SwitchToMerchant converts a regular user to a merchant by creating a merchant profile.
func (srv *profileService) SwitchToMerchant(ctx context.Context, userID uuid.UUID, input *usecase.SwitchToMerchantInput) error {
	srv.log(ctx).Info("Switching user to merchant", slog.String("user_id", userID.String()))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		// 1. Find the user
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrNotFound)
			}

			return err
		}

		// 2. Check if user already has a merchant profile
		if user.MerchantProfile != nil {
			return domainerrors.ErrConflict.WithDetails("user already has a merchant profile")
		}

		// 3. Create merchant profile
		user.MerchantProfile = &entity.MerchantProfile{
			StoreName:          input.StoreName,
			VerificationStatus: entity.MerchantVerificationStatusUnverified,
		}

		// 4. Save the updated user
		if err := userRepo.Update(ctx, user); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("failed to switch user to merchant", slog.String("error", err.Error()))

		return err
	}
	srv.log(ctx).Debug("user switched to merchant", slog.String("user_id", userID.String()))

	return nil
}

func validateMerchantDiscoveryValues(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	categoryID *uuid.UUID,
	subcategoryID *uuid.UUID,
	hubID *uuid.UUID,
) error {
	var category *entity.DiscoveryCategory
	if categoryID != nil {
		foundCategory, err := validateActiveDiscoveryCategory(ctx, discoveryRepo, *categoryID)
		if err != nil {
			return err
		}
		category = foundCategory
	}

	if subcategoryID != nil {
		if err := validateRequiredActiveDiscoverySubcategory(ctx, discoveryRepo, category, *subcategoryID); err != nil {
			return err
		}
	}

	if hubID != nil {
		if err := validateActiveHub(ctx, discoveryRepo, *hubID); err != nil {
			return err
		}
	}

	return nil
}

func validateMerchantDiscoveryUpdateValues(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
	update merchantDiscoveryProfileUpdate,
) error {
	if update.isPublic {
		return validateMerchantDiscoveryValues(ctx, discoveryRepo, update.categoryID, update.subcategoryID, update.hubID)
	}

	return validateSelectedMerchantDiscoveryValues(ctx, discoveryRepo, input, update)
}

func validateSelectedMerchantDiscoveryValues(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
	update merchantDiscoveryProfileUpdate,
) error {
	if err := validateSelectedDiscoveryCategory(ctx, discoveryRepo, input, update.categoryID); err != nil {
		return err
	}
	if err := validateSelectedDiscoverySubcategory(ctx, discoveryRepo, input, update); err != nil {
		return err
	}
	if err := validateRetainedDiscoverySubcategory(ctx, discoveryRepo, input, update); err != nil {
		return err
	}
	if err := validateSelectedHub(ctx, discoveryRepo, input, update.hubID); err != nil {
		return err
	}

	return nil
}

func validateActiveDiscoveryCategory(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	categoryID uuid.UUID,
) (*entity.DiscoveryCategory, error) {
	category, err := discoveryRepo.FindCategoryByID(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if category.Status != entity.DiscoveryStatusActive {
		return nil, domainerrors.ErrValidationFailed.WithDetails("discovery_category_id must reference an active category")
	}

	return category, nil
}

func validateRequiredActiveDiscoverySubcategory(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	category *entity.DiscoveryCategory,
	subcategoryID uuid.UUID,
) error {
	if category == nil {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_subcategory_id requires discovery_category_id")
	}

	return validateActiveDiscoverySubcategory(ctx, discoveryRepo, category.ID, subcategoryID)
}

func validateActiveDiscoverySubcategory(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	categoryID uuid.UUID,
	subcategoryID uuid.UUID,
) error {
	subcategory, err := discoveryRepo.FindSubcategoryByID(ctx, subcategoryID)
	if err != nil {
		return err
	}
	if subcategory.Status != entity.DiscoveryStatusActive {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_subcategory_id must reference an active subcategory")
	}
	if subcategory.CategoryID != categoryID {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_subcategory_id must belong to discovery_category_id")
	}

	return nil
}

func validateActiveHub(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	hubID uuid.UUID,
) error {
	hub, err := discoveryRepo.FindHubByID(ctx, hubID)
	if err != nil {
		return err
	}
	if hub.Status != entity.DiscoveryStatusActive {
		return domainerrors.ErrValidationFailed.WithDetails("active_hub_id must reference an active hub")
	}

	return nil
}

func validateSelectedDiscoveryCategory(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
	categoryID *uuid.UUID,
) error {
	if !input.DiscoveryCategoryID.IsSet || categoryID == nil {
		return nil
	}

	_, err := validateActiveDiscoveryCategory(ctx, discoveryRepo, *categoryID)

	return err
}

func validateSelectedDiscoverySubcategory(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
	update merchantDiscoveryProfileUpdate,
) error {
	if !input.DiscoverySubcategoryID.IsSet || update.subcategoryID == nil {
		return nil
	}
	if update.categoryID == nil {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_subcategory_id requires discovery_category_id")
	}

	return validateActiveDiscoverySubcategory(ctx, discoveryRepo, *update.categoryID, *update.subcategoryID)
}

func validateRetainedDiscoverySubcategory(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
	update merchantDiscoveryProfileUpdate,
) error {
	if !shouldValidateRetainedDiscoverySubcategory(input, update.subcategoryID) {
		return nil
	}
	if update.categoryID == nil {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_subcategory_id requires discovery_category_id")
	}

	return validateDiscoverySubcategoryCategory(ctx, discoveryRepo, *update.categoryID, *update.subcategoryID)
}

func shouldValidateRetainedDiscoverySubcategory(
	input *usecase.UpdateMerchantDiscoveryProfileInput,
	subcategoryID *uuid.UUID,
) bool {
	return input.DiscoveryCategoryID.IsSet && !input.DiscoverySubcategoryID.IsSet && subcategoryID != nil
}

func validateDiscoverySubcategoryCategory(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	categoryID uuid.UUID,
	subcategoryID uuid.UUID,
) error {
	subcategory, err := discoveryRepo.FindSubcategoryByID(ctx, subcategoryID)
	if err != nil {
		return err
	}
	if subcategory.CategoryID != categoryID {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_subcategory_id must belong to discovery_category_id")
	}

	return nil
}

func validateSelectedHub(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	input *usecase.UpdateMerchantDiscoveryProfileInput,
	hubID *uuid.UUID,
) error {
	if !input.ActiveHubID.IsSet || hubID == nil {
		return nil
	}

	return validateActiveHub(ctx, discoveryRepo, *hubID)
}

func validateMerchantDiscoveryEligibility(
	profile *entity.MerchantProfile,
	categoryID *uuid.UUID,
	subcategoryID *uuid.UUID,
	hasActivePrimaryLocation bool,
) error {
	if profile.VerificationStatus != entity.MerchantVerificationStatusVerified {
		return domainerrors.ErrValidationFailed.WithDetails("merchant must be verified before enabling public discovery")
	}
	if categoryID == nil {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_category_id is required before enabling public discovery")
	}
	if subcategoryID == nil {
		return domainerrors.ErrValidationFailed.WithDetails("discovery_subcategory_id is required before enabling public discovery")
	}
	if !hasActivePrimaryLocation {
		return domainerrors.ErrValidationFailed.WithDetails("active primary merchant location is required before enabling public discovery")
	}

	return nil
}

func merchantHasActivePrimaryLocation(
	ctx context.Context,
	addressRepo repository.AddressRepository,
	merchantID uuid.UUID,
) (bool, error) {
	address, err := addressRepo.FindPrimaryAddressByOwner(ctx, merchantID, entity.OwnerTypeMerchantProfile)
	if err != nil {
		if errors.Is(err, domainerrors.ErrAddressNotFound) {
			return false, nil
		}

		return false, err
	}
	if address == nil {
		return false, nil
	}

	return address.IsActive, nil
}

func buildMerchantDiscoveryProfileResult(
	ctx context.Context,
	discoveryRepo repository.DiscoveryRepository,
	profile *entity.MerchantProfile,
	hasActivePrimaryLocation bool,
) (*usecase.MerchantDiscoveryProfileResult, error) {
	result := &usecase.MerchantDiscoveryProfileResult{
		DiscoveryCategoryID:      profile.DiscoveryCategoryID,
		DiscoverySubcategoryID:   profile.DiscoverySubcategoryID,
		ActiveHubID:              profile.ActiveHubID,
		IsPublic:                 profile.IsPublic,
		IsVerified:               profile.VerificationStatus == entity.MerchantVerificationStatusVerified,
		HasActivePrimaryLocation: hasActivePrimaryLocation,
	}

	if profile.DiscoveryCategoryID != nil {
		category, err := discoveryRepo.FindCategoryByID(ctx, *profile.DiscoveryCategoryID)
		if err != nil {
			return nil, err
		}
		result.DiscoveryCategory = category
	}
	if profile.DiscoverySubcategoryID != nil {
		subcategory, err := discoveryRepo.FindSubcategoryByID(ctx, *profile.DiscoverySubcategoryID)
		if err != nil {
			return nil, err
		}
		result.DiscoverySubcategory = subcategory
	}
	if profile.ActiveHubID != nil {
		hub, err := discoveryRepo.FindHubByID(ctx, *profile.ActiveHubID)
		if err != nil {
			return nil, err
		}
		result.ActiveHub = hub
	}

	return result, nil
}

// GetUserRole returns the roles associated with a user.
func (srv *profileService) GetUserRole(ctx context.Context, userID uuid.UUID) ([]string, error) {
	srv.log(ctx).Debug("Getting user roles", slog.String("user_id", userID.String()))

	var roles entity.Roles

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return replaceWithSourceStack(err, domainerrors.ErrNotFound)
			}

			return err
		}

		// Extract roles based on profiles
		if user.UserProfile != nil {
			roles = append(roles, entity.RoleUser)
		}
		if user.MerchantProfile != nil {
			roles = append(roles, entity.RoleMerchant)
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("failed to get user roles", slog.String("error", err.Error()))

		return nil, err
	}
	srv.log(ctx).Debug("user roles", slog.Any("roles", roles))

	return roles.ToStrings(), nil
}
