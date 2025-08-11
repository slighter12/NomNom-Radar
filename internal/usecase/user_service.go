// Package usecase contains the implementation of the application's business logic.
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"radar/internal/domain/entity"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"

	"go.uber.org/fx"
)

// userService implements the UserUsecase interface.
type userService struct {
	fx.In

	txManager    repository.TransactionManager
	hasher       service.PasswordHasher
	tokenService service.TokenService
	logger       *slog.Logger
}

// NewUserService is the constructor for userService. It receives all dependencies as interfaces.
func NewUserService(
	txManager repository.TransactionManager,
	hasher service.PasswordHasher,
	tokenService service.TokenService,
	logger *slog.Logger,
) UserUsecase {
	return &userService{
		txManager:    txManager,
		hasher:       hasher,
		tokenService: tokenService,
		logger:       logger,
	}
}

// RegisterUser orchestrates the complete user registration process.
func (s *userService) RegisterUser(ctx context.Context, input RegisterUserInput) (*RegisterOutput, error) {
	s.logger.Info("Starting user registration", "email", input.Email)

	hashedPassword, err := s.hasher.Hash(input.Password)
	if err != nil {
		s.logger.Error("Failed to hash password during registration", "error", err)
		return nil, errors.New("internal server error")
	}

	var registeredUser *entity.User

	// Execute the entire creation process within a single database transaction
	// to ensure data consistency (atomicity).
	err = s.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		authRepo := repoFactory.NewAuthRepository()

		// 1. Check if an authentication method with this email already exists.
		_, err := authRepo.FindAuthentication(ctx, "email", input.Email)
		if err == nil {
			// If no error, it means an auth record was found.
			return errors.New("user with this email already exists")
		}
		// We expect a 'not found' error. If it's a different error, something went wrong.
		if !errors.Is(err, repository.ErrAuthNotFound) {
			return err
		}

		// 2. Create the User entity and its associated UserProfile.
		newUser := &entity.User{
			Name:        input.Name,
			Email:       input.Email,
			UserProfile: &entity.UserProfile{}, // Create an empty profile for the user role.
		}

		if err := userRepo.Create(ctx, newUser); err != nil {
			return err
		}

		// 3. Create the Authentication entity (the email/password credential).
		newAuth := &entity.Authentication{
			UserID:         newUser.ID,
			Provider:       "email",
			ProviderUserID: input.Email,
			PasswordHash:   hashedPassword,
		}
		if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
			return err
		}

		registeredUser = newUser
		return nil // Returning nil commits the transaction.
	})

	if err != nil {
		s.logger.Error("Failed to execute user registration transaction", "error", err, "email", input.Email)
		return nil, err
	}

	s.logger.Debug("User registered successfully", "userID", registeredUser.ID)
	return &RegisterOutput{User: registeredUser}, nil
}

// RegisterMerchant would follow a very similar transactional pattern.
func (s *userService) RegisterMerchant(ctx context.Context, input RegisterMerchantInput) (*RegisterOutput, error) {
	// ... Implementation would be very similar to RegisterUser,
	// but it would create an entity.MerchantProfile instead.
	return nil, errors.New("not implemented")
}

// Login orchestrates the user login process.
func (s *userService) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	s.logger.Debug("Starting user login", "email", input.Email)

	var loggedInUser *entity.User
	var roles []string
	var accessToken, refreshTokenString string

	// Login involves multiple steps, so we use a transaction to ensure atomicity,
	// especially for creating the new refresh token.
	err := s.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.NewAuthRepository()
		userRepo := repoFactory.NewUserRepository()

		// 1. Find the authentication method.
		authRecord, err := authRepo.FindAuthentication(ctx, "email", input.Email)
		if err != nil {
			// This includes ErrAuthNotFound, which we can treat as an invalid credential case.
			return errors.New("invalid email or password")
		}

		// 2. Check the password.
		if !s.hasher.Check(input.Password, authRecord.PasswordHash) {
			return errors.New("invalid email or password")
		}

		// 3. Fetch the full user and profile data to determine roles.
		user, err := userRepo.FindByID(ctx, authRecord.UserID)
		if err != nil {
			return err
		}

		if user.UserProfile != nil {
			roles = append(roles, "user")
		}
		if user.MerchantProfile != nil {
			roles = append(roles, "merchant")
		}

		// 4. Generate new tokens.
		accessToken, refreshTokenString, err = s.tokenService.GenerateTokens(user.ID, roles)
		if err != nil {
			return err
		}

		// 5. Securely store the new refresh token.
		hasher := sha256.New()
		hasher.Write([]byte(refreshTokenString))
		refreshTokenHash := hex.EncodeToString(hasher.Sum(nil))

		newRefreshToken := &entity.RefreshToken{
			UserID:    user.ID,
			TokenHash: refreshTokenHash,
			ExpiresAt: time.Now().Add(s.tokenService.GetRefreshTokenDuration()),
		}

		if err := authRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
			return err
		}

		loggedInUser = user
		return nil
	})

	if err != nil {
		s.logger.Warn("Login failed", "email", input.Email, "error", err.Error())
		return nil, err
	}

	s.logger.Debug("User logged in successfully", "userID", loggedInUser.ID)
	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		User:         loggedInUser,
	}, nil
}
