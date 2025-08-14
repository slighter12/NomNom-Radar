// Package usecase contains the implementation of the application's business logic.
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/fx"
)

// userService implements the UserUsecase interface.
type userService struct {
	fx.In

	txManager         repository.TransactionManager
	hasher            service.PasswordHasher
	tokenService      service.TokenService
	googleAuthService service.OAuthAuthService // Updated to use OAuthAuthService
	logger            *slog.Logger
}

// NewUserService is the constructor for userService. It receives all dependencies as interfaces.
func NewUserService(
	txManager repository.TransactionManager,
	hasher service.PasswordHasher,
	tokenService service.TokenService,
	googleAuthService service.OAuthAuthService, // Updated to use OAuthAuthService
	logger *slog.Logger,
) UserUsecase {
	return &userService{
		txManager:         txManager,
		hasher:            hasher,
		tokenService:      tokenService,
		googleAuthService: googleAuthService,
		logger:            logger,
	}
}

// RegisterUser orchestrates the complete user registration process.
func (srv *userService) RegisterUser(ctx context.Context, input *RegisterUserInput) (*RegisterOutput, error) {
	srv.logger.Info("Starting user registration", "email", input.Email)

	hashedPassword, err := srv.hasher.Hash(input.Password)
	if err != nil {
		srv.logger.Error("Failed to hash password during registration", "error", err)

		return nil, errors.Wrap(err, "failed to hash password during registration")
	}

	var registeredUser *entity.User

	// Execute the entire creation process within a single database transaction
	// to ensure data consistency (atomicity).
	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		authRepo := repoFactory.NewAuthRepository()

		// 1. Check if an authentication method with this email already exists.
		_, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email)
		if err == nil {
			// If no error, it means an auth record was found.
			return domainerrors.ErrUserAlreadyExists.WrapMessage("user registration failed")
		}
		// We expect a 'not found' error. If it's a different error, something went wrong.
		if !errors.Is(err, repository.ErrAuthNotFound) {
			return errors.Wrap(err, "failed to find authentication")
		}

		// 2. Create the User entity and its associated UserProfile.
		newUser := &entity.User{
			Name:        input.Name,
			Email:       input.Email,
			UserProfile: &entity.UserProfile{}, // Create an empty profile for the user role.
		}

		if err := userRepo.Create(ctx, newUser); err != nil {
			return errors.WithStack(err)
		}

		// 3. Create the Authentication entity (the email/password credential).
		newAuth := &entity.Authentication{
			UserID:         newUser.ID,
			Provider:       entity.ProviderTypeEmail,
			ProviderUserID: input.Email,
			PasswordHash:   hashedPassword,
		}
		if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
			return errors.WithStack(err)
		}
		registeredUser = newUser

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to execute user registration transaction", "error", err, "email", input.Email)

		return nil, errors.Wrap(err, "failed to execute user registration transaction")
	}
	srv.logger.Debug("User registered successfully", "userID", registeredUser.ID)

	return &RegisterOutput{User: registeredUser}, nil
}

// RegisterMerchant orchestrates the complete merchant registration process.
func (srv *userService) RegisterMerchant(ctx context.Context, input *RegisterMerchantInput) (*RegisterOutput, error) {
	srv.logger.Info("Starting merchant registration", "email", input.Email)

	hashedPassword, err := srv.hasher.Hash(input.Password)
	if err != nil {
		srv.logger.Error("Failed to hash password during registration", "error", err)

		return nil, errors.Wrap(err, "failed to hash password during registration")
	}

	var registeredUser *entity.User

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		authRepo := repoFactory.NewAuthRepository()

		// 1. Check if an authentication method with this email already exists.
		_, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email)
		if err == nil {
			return errors.Wrap(domainerrors.ErrMerchantAlreadyExists, "merchant registration failed")
		}
		if !errors.Is(err, repository.ErrAuthNotFound) {
			return errors.Wrap(err, "failed to find authentication")
		}

		// 2. Create the User entity and its associated MerchantProfile.
		newUser := &entity.User{
			Name:  input.Name,
			Email: input.Email,
			MerchantProfile: &entity.MerchantProfile{
				StoreName:       input.StoreName,
				BusinessLicense: input.BusinessLicense,
			},
		}

		if err := userRepo.Create(ctx, newUser); err != nil {
			return errors.WithStack(err)
		}

		// 3. Create the Authentication entity.
		newAuth := &entity.Authentication{
			UserID:         newUser.ID,
			Provider:       entity.ProviderTypeEmail,
			ProviderUserID: input.Email,
			PasswordHash:   hashedPassword,
		}
		if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
			return errors.WithStack(err)
		}
		registeredUser = newUser

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to execute merchant registration transaction", "error", err, "email", input.Email)

		return nil, errors.Wrap(err, "failed to execute merchant registration transaction")
	}
	srv.logger.Debug("Merchant registered successfully", "userID", registeredUser.ID)

	return &RegisterOutput{User: registeredUser}, nil
}

// Login orchestrates the user login process.
func (srv *userService) Login(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
	srv.logger.Debug("Starting user login", "email", input.Email)

	var loggedInUser *entity.User
	var roles []string
	var accessToken, refreshTokenString string

	// Login involves multiple steps, so we use a transaction to ensure atomicity,
	// especially for creating the new refresh token.
	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.NewAuthRepository()
		userRepo := repoFactory.NewUserRepository()

		// 1. Find the authentication method.
		authRecord, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeEmail, input.Email)
		if err != nil {
			// This includes ErrAuthNotFound, which we can treat as an invalid credential case.
			return errors.Wrap(domainerrors.ErrInvalidCredentials, "login failed")
		}

		// 2. Check the password.
		if !srv.hasher.Check(input.Password, authRecord.PasswordHash) {
			return errors.Wrap(domainerrors.ErrInvalidCredentials, "login failed")
		}

		// 3. Fetch the full user and profile data to determine roles.
		user, err := userRepo.FindByID(ctx, authRecord.UserID)
		if err != nil {
			return errors.Wrap(err, "failed to find user by id")
		}

		if user.UserProfile != nil {
			roles = append(roles, "user")
		}
		if user.MerchantProfile != nil {
			roles = append(roles, "merchant")
		}

		// 4. Generate new tokens.
		accessToken, refreshTokenString, err = srv.tokenService.GenerateTokens(user.ID, roles)
		if err != nil {
			return errors.Wrap(err, "failed to generate tokens")
		}

		// 5. Securely store the new refresh token.
		hasher := sha256.New()
		hasher.Write([]byte(refreshTokenString))
		refreshTokenHash := hex.EncodeToString(hasher.Sum(nil))

		newRefreshToken := &entity.RefreshToken{
			UserID:    user.ID,
			TokenHash: refreshTokenHash,
			ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
		}

		if err := authRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
			return errors.WithStack(err)
		}
		loggedInUser = user

		return nil
	})

	if err != nil {
		srv.logger.Warn("Login failed", "email", input.Email, "error", err.Error())

		return nil, errors.Wrap(err, "failed to execute user login transaction")
	}
	srv.logger.Debug("User logged in successfully", "userID", loggedInUser.ID)

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		User:         loggedInUser,
	}, nil
}

// RefreshToken handles the process of issuing a new token pair using a refresh token.
func (srv *userService) RefreshToken(ctx context.Context, input *RefreshTokenInput) (*RefreshTokenOutput, error) {
	srv.logger.Info("Attempting to refresh token")

	claims, err := srv.tokenService.ValidateToken(input.RefreshToken)
	if err != nil {
		return nil, errors.Wrap(err, "invalid refresh token")
	}

	var newAccessToken, newRefreshTokenString string

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.NewAuthRepository()
		userRepo := repoFactory.NewUserRepository()

		// 1. Verify the refresh token exists in the database.
		hasher := sha256.New()
		hasher.Write([]byte(input.RefreshToken))
		tokenHash := hex.EncodeToString(hasher.Sum(nil))

		_, err := authRepo.FindRefreshTokenByHash(ctx, tokenHash)
		if err != nil {
			return errors.Wrap(err, "refresh token not found or expired")
		}

		// 2. Fetch user and roles.
		user, err := userRepo.FindByID(ctx, claims.UserID)
		if err != nil {
			return errors.Wrap(err, "failed to find user")
		}
		var roles []string
		if user.UserProfile != nil {
			roles = append(roles, "user")
		}
		if user.MerchantProfile != nil {
			roles = append(roles, "merchant")
		}

		// 3. Generate new tokens.
		newAccessToken, newRefreshTokenString, err = srv.tokenService.GenerateTokens(user.ID, roles)
		if err != nil {
			return errors.Wrap(err, "failed to generate new tokens")
		}

		// 4. Store the new refresh token.
		newHasher := sha256.New()
		newHasher.Write([]byte(newRefreshTokenString))
		newRefreshTokenHash := hex.EncodeToString(newHasher.Sum(nil))

		newRefreshToken := &entity.RefreshToken{
			UserID:    user.ID,
			TokenHash: newRefreshTokenHash,
			ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
		}
		if err := authRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
			return errors.WithStack(err)
		}

		// 5. Delete the old refresh token.
		if err := authRepo.DeleteRefreshTokenByHash(ctx, tokenHash); err != nil {
			// Log the error but don't fail the transaction, as the user has a new valid token.
			srv.logger.Warn("Failed to delete old refresh token", "error", err)
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to execute refresh token transaction", "error", err)

		return nil, errors.Wrap(err, "failed to execute refresh token transaction")
	}

	return &RefreshTokenOutput{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshTokenString,
	}, nil
}

// Logout handles the process of invalidating a user's session by deleting their refresh token.
func (srv *userService) Logout(ctx context.Context, input *LogoutInput) error {
	srv.logger.Info("Attempting to log out")

	_, err := srv.tokenService.ValidateToken(input.RefreshToken)
	if err != nil {
		// Even if the token is invalid, we can proceed to delete it from the database.
		srv.logger.Warn("Logout with invalid token", "error", err)
	}

	hasher := sha256.New()
	hasher.Write([]byte(input.RefreshToken))
	tokenHash := hex.EncodeToString(hasher.Sum(nil))

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.NewAuthRepository()
		if err := authRepo.DeleteRefreshTokenByHash(ctx, tokenHash); err != nil {
			return errors.Wrap(err, "failed to delete refresh token")
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to execute logout transaction", "error", err)

		return errors.Wrap(err, "failed to execute logout transaction")
	}
	srv.logger.Info("Successfully logged out")

	return nil
}

// GoogleCallback handles the user login or registration via Google Sign-In.
func (srv *userService) GoogleCallback(ctx context.Context, input *GoogleCallbackInput) (*LoginOutput, error) {
	srv.logger.Info("Handling Google callback")

	// 1. Verify the ID token with Google.
	oauthUser, err := srv.googleAuthService.VerifyToken(ctx, input.IDToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to verify Google ID token")
	}

	// 2. Find or create user and generate tokens
	loggedInUser, accessToken, refreshTokenString, err := srv.handleGoogleUserAuth(ctx, oauthUser)
	if err != nil {
		return nil, errors.Wrap(err, "failed to handle Google user authentication")
	}

	return &LoginOutput{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		User:         loggedInUser,
	}, nil
}

// handleGoogleUserAuth handles the complete Google user authentication flow
func (srv *userService) handleGoogleUserAuth(ctx context.Context, oauthUser *service.OAuthUser) (*entity.User, string, string, error) {
	var loggedInUser *entity.User
	var accessToken, refreshTokenString string

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		// Find or create user
		user, err := srv.findOrCreateGoogleUser(ctx, repoFactory, oauthUser)
		if err != nil {
			return err
		}
		loggedInUser = user

		// Generate tokens
		accessToken, refreshTokenString, err = srv.generateUserTokens(ctx, user)
		if err != nil {
			return errors.Wrap(err, "failed to find or create Google user")
		}

		// Store refresh token
		return srv.storeRefreshToken(ctx, repoFactory, user.ID, refreshTokenString)
	})

	if err != nil {
		return nil, "", "", errors.Wrap(err, "failed to execute Google user authentication transaction")
	}

	return loggedInUser, accessToken, refreshTokenString, nil
}

// findOrCreateGoogleUser finds existing user or creates new one for Google authentication
func (srv *userService) findOrCreateGoogleUser(ctx context.Context, repoFactory repository.RepositoryFactory, oauthUser *service.OAuthUser) (*entity.User, error) {
	authRepo := repoFactory.NewAuthRepository()
	userRepo := repoFactory.NewUserRepository()

	// Try to find existing authentication record
	authRecord, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeGoogle, oauthUser.ID)
	if err != nil && !errors.Is(err, repository.ErrAuthNotFound) {
		return nil, errors.Wrap(err, "failed to find authentication")
	}

	// If user doesn't exist, create new one
	if errors.Is(err, repository.ErrAuthNotFound) {
		return srv.createGoogleUser(ctx, userRepo, authRepo, oauthUser)
	}

	// If user exists, fetch their data
	return srv.fetchExistingUser(ctx, userRepo, authRecord.UserID)
}

// createGoogleUser creates a new user for Google authentication
func (srv *userService) createGoogleUser(ctx context.Context, userRepo repository.UserRepository, authRepo repository.AuthRepository, oauthUser *service.OAuthUser) (*entity.User, error) {
	srv.logger.Info("Google user not found, creating new user", "email", oauthUser.Email)

	newUser := &entity.User{
		Name:        oauthUser.Name,
		Email:       oauthUser.Email,
		UserProfile: &entity.UserProfile{}, // Default role is 'user'
	}

	if err := userRepo.Create(ctx, newUser); err != nil {
		return nil, errors.WithStack(err)
	}

	newAuth := &entity.Authentication{
		UserID:         newUser.ID,
		Provider:       entity.ProviderTypeGoogle,
		ProviderUserID: oauthUser.ID,
	}

	if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
		return nil, errors.WithStack(err)
	}

	return newUser, nil
}

// fetchExistingUser fetches existing user by ID
func (srv *userService) fetchExistingUser(ctx context.Context, userRepo repository.UserRepository, userID uuid.UUID) (*entity.User, error) {
	srv.logger.Info("Found existing Google user", "userID", userID)

	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user by id for google auth")
	}

	return user, nil
}

// generateUserTokens generates access and refresh tokens for the user
func (srv *userService) generateUserTokens(_ context.Context, user *entity.User) (string, string, error) {
	roles := srv.extractUserRoles(user)

	accessToken, refreshTokenString, err := srv.tokenService.GenerateTokens(user.ID, roles)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to generate tokens for google auth")
	}

	return accessToken, refreshTokenString, nil
}

// extractUserRoles extracts roles from user profiles
func (srv *userService) extractUserRoles(user *entity.User) []string {
	var roles []string

	if user.UserProfile != nil {
		roles = append(roles, "user")
	}
	if user.MerchantProfile != nil {
		roles = append(roles, "merchant")
	}

	return roles
}

// storeRefreshToken stores the refresh token in the database
func (srv *userService) storeRefreshToken(ctx context.Context, repoFactory repository.RepositoryFactory, userID uuid.UUID, refreshTokenString string) error {
	authRepo := repoFactory.NewAuthRepository()

	// Hash the refresh token
	hasher := sha256.New()
	hasher.Write([]byte(refreshTokenString))
	refreshTokenHash := hex.EncodeToString(hasher.Sum(nil))

	newRefreshToken := &entity.RefreshToken{
		UserID:    userID,
		TokenHash: refreshTokenHash,
		ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
	}

	if err := authRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
