// Package impl contains the application-specific business rules implementations.
package impl

import (
	"context"
	"log/slog"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
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

// GetProfile retrieves the complete user profile including role-specific data.
func (srv *profileService) GetProfile(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	srv.logger.Debug("Getting user profile", "userID", userID)

	var user *entity.User

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		foundUser, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}
		user = foundUser

		return nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "failed to get user profile")
	}

	return user, nil
}

// UpdateUserProfile updates the user profile data.
func (srv *profileService) UpdateUserProfile(ctx context.Context, userID uuid.UUID, input *usecase.UpdateUserProfileInput) error {
	srv.logger.Info("Updating user profile", "userID", userID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		// 1. Find the user
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Check if user has a user profile
		if user.UserProfile == nil {
			return errors.Wrap(domainerrors.ErrValidationFailed, "user does not have a user profile")
		}

		// 3. Update the profile fields
		if input.LoyaltyPoints != nil {
			user.UserProfile.LoyaltyPoints = *input.LoyaltyPoints
		}

		// 4. Save the updated user
		if err := userRepo.Update(ctx, user); err != nil {
			return errors.Wrap(err, "failed to update user profile")
		}

		return nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to update user profile")
	}

	return nil
}

// UpdateMerchantProfile updates the merchant profile data.
func (srv *profileService) UpdateMerchantProfile(ctx context.Context, userID uuid.UUID, input *usecase.UpdateMerchantProfileInput) error {
	srv.logger.Info("Updating merchant profile", "userID", userID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		// 1. Find the user
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Check if user has a merchant profile
		if user.MerchantProfile == nil {
			return errors.Wrap(domainerrors.ErrValidationFailed, "user does not have a merchant profile")
		}

		// 3. Update the profile fields
		if input.StoreName != nil {
			user.MerchantProfile.StoreName = *input.StoreName
		}
		if input.StoreDescription != nil {
			user.MerchantProfile.StoreDescription = *input.StoreDescription
		}
		if input.BusinessLicense != nil {
			user.MerchantProfile.BusinessLicense = *input.BusinessLicense
		}

		// 4. Save the updated user
		if err := userRepo.Update(ctx, user); err != nil {
			return errors.Wrap(err, "failed to update merchant profile")
		}

		return nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to update merchant profile")
	}

	return nil
}

// SwitchToMerchant converts a regular user to a merchant by creating a merchant profile.
func (srv *profileService) SwitchToMerchant(ctx context.Context, userID uuid.UUID, input *usecase.SwitchToMerchantInput) error {
	srv.logger.Info("Switching user to merchant", "userID", userID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		// 1. Find the user
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		// 2. Check if user already has a merchant profile
		if user.MerchantProfile != nil {
			return errors.Wrap(domainerrors.ErrConflict, "user already has a merchant profile")
		}

		// 3. Create merchant profile
		user.MerchantProfile = &entity.MerchantProfile{
			StoreName:       input.StoreName,
			BusinessLicense: input.BusinessLicense,
		}

		// 4. Save the updated user
		if err := userRepo.Update(ctx, user); err != nil {
			return errors.Wrap(err, "failed to create merchant profile")
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("failed to switch user to merchant", "error", err)

		return errors.Wrap(err, "failed to switch user to merchant")
	}
	srv.logger.Debug("user switched to merchant", "userID", userID)

	return nil
}

// GetUserRole returns the roles associated with a user.
func (srv *profileService) GetUserRole(ctx context.Context, userID uuid.UUID) ([]string, error) {
	srv.logger.Debug("Getting user roles", "userID", userID)

	var roles entity.Roles

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
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
		srv.logger.Error("failed to get user roles", "error", err)

		return nil, errors.Wrap(err, "failed to get user roles")
	}
	srv.logger.Debug("user roles", "roles", roles)

	return roles.ToStrings(), nil
}
