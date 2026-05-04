package impl

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	deliverycontext "radar/internal/delivery/context"
	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

// profileService implements the ProfileUsecase interface.
type profileService struct {
	fx.In

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
	return deliverycontext.GetLoggerOrDefault(ctx, srv.logger)
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
				return domainerrors.ErrNotFound
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
				return domainerrors.ErrNotFound
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
				return domainerrors.ErrNotFound
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
				return domainerrors.ErrNotFound
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
				return domainerrors.ErrNotFound
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

// GetUserRole returns the roles associated with a user.
func (srv *profileService) GetUserRole(ctx context.Context, userID uuid.UUID) ([]string, error) {
	srv.log(ctx).Debug("Getting user roles", slog.String("user_id", userID.String()))

	var roles entity.Roles

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, domainerrors.ErrUserNotFound) {
				return domainerrors.ErrNotFound
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
