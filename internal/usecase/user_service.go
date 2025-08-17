// Package usecase contains the implementation of the application's business logic.
package usecase

import (
	"context"
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

	txManager repository.TransactionManager
	// Direct repository instances for non-transactional operations
	userRepo          repository.UserRepository
	authRepo          repository.AuthRepository
	refreshTokenRepo  repository.RefreshTokenRepository
	hasher            service.PasswordHasher
	tokenService      service.TokenService
	googleAuthService service.OAuthAuthService
	logger            *slog.Logger
}

// NewUserService is the constructor for userService. It receives all dependencies as interfaces.
func NewUserService(
	txManager repository.TransactionManager,
	userRepo repository.UserRepository,
	authRepo repository.AuthRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	hasher service.PasswordHasher,
	tokenService service.TokenService,
	googleAuthService service.OAuthAuthService,
	logger *slog.Logger,
) UserUsecase {
	return &userService{
		txManager:         txManager,
		userRepo:          userRepo,
		authRepo:          authRepo,
		refreshTokenRepo:  refreshTokenRepo,
		hasher:            hasher,
		tokenService:      tokenService,
		googleAuthService: googleAuthService,
		logger:            logger,
	}
}

// RegisterUser orchestrates the complete user registration process.
func (srv *userService) RegisterUser(ctx context.Context, input *RegisterUserInput) (*RegisterOutput, error) {
	srv.logger.Info("Starting user registration", "email", input.Email)

	// Validate password strength
	if err := srv.hasher.ValidatePasswordStrength(input.Password); err != nil {
		srv.logger.Warn("Password validation failed during registration", "email", input.Email, "error", err)
		return nil, errors.Wrap(domainerrors.ErrValidationFailed, "password does not meet security requirements")
	}

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

	// Validate password strength
	if err := srv.hasher.ValidatePasswordStrength(input.Password); err != nil {
		srv.logger.Warn("Password validation failed during merchant registration", "email", input.Email, "error", err)
		return nil, errors.Wrap(domainerrors.ErrValidationFailed, "password does not meet security requirements")
	}

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
		refreshRepo := repoFactory.NewRefreshTokenRepository()

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
		refreshTokenHash := srv.tokenService.HashToken(refreshTokenString)

		newRefreshToken := &entity.RefreshToken{
			UserID:    user.ID,
			TokenHash: refreshTokenHash,
			ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
		}

		if err := refreshRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
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

// RefreshToken handles the process of issuing a new access token using a refresh token.
// The refresh token remains unchanged for security reasons.
func (srv *userService) RefreshToken(ctx context.Context, input *RefreshTokenInput) (*RefreshTokenOutput, error) {
	srv.logger.Info("Attempting to refresh access token")

	claims, err := srv.tokenService.ValidateToken(input.RefreshToken)
	if err != nil {
		return nil, errors.Wrap(err, "invalid refresh token")
	}

	var newAccessToken string

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.NewUserRepository()
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// 1. Verify the refresh token exists in the database.
		tokenHash := srv.tokenService.HashToken(input.RefreshToken)

		_, err := refreshRepo.FindRefreshTokenByHash(ctx, tokenHash)
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

		// 3. Generate only a new access token (refresh token remains unchanged).
		newAccessToken, _, err = srv.tokenService.GenerateTokens(user.ID, roles)
		if err != nil {
			return errors.Wrap(err, "failed to generate new access token")
		}

		// Note: We don't modify the refresh token - it remains valid and unchanged
		// This prevents token rotation attacks and maintains better security

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to execute refresh token transaction", "error", err)
		return nil, errors.Wrap(err, "failed to execute refresh token transaction")
	}

	return &RefreshTokenOutput{
		AccessToken: newAccessToken,
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

	tokenHash := srv.tokenService.HashToken(input.RefreshToken)

	// Single operation - use direct repository instance
	if err := srv.refreshTokenRepo.DeleteRefreshTokenByHash(ctx, tokenHash); err != nil {
		srv.logger.Error("Failed to delete refresh token", "error", err)
		return errors.Wrap(err, "failed to delete refresh token")
	}

	srv.logger.Info("Successfully logged out")
	return nil
}

// GoogleCallback handles the user login or registration via Google Sign-In.
func (srv *userService) GoogleCallback(ctx context.Context, input *GoogleCallbackInput) (*LoginOutput, error) {
	srv.logger.Info("Handling Google callback")

	// 1. Verify the ID token with Google.
	oauthUser, err := srv.googleAuthService.VerifyIDToken(ctx, input.IDToken)
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
	refreshRepo := repoFactory.NewRefreshTokenRepository()

	// Hash the refresh token
	refreshTokenHash := srv.tokenService.HashToken(refreshTokenString)

	newRefreshToken := &entity.RefreshToken{
		UserID:    userID,
		TokenHash: refreshTokenHash,
		ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
	}

	if err := refreshRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// LogoutAllDevices handles the process of invalidating all user sessions by deleting all refresh tokens.
func (srv *userService) LogoutAllDevices(ctx context.Context, userID uuid.UUID) error {
	srv.logger.Info("Attempting to log out from all devices", "userID", userID)

	// Single operation - use direct repository instance
	if err := srv.refreshTokenRepo.DeleteRefreshTokensByUserID(ctx, userID); err != nil {
		srv.logger.Error("Failed to delete all refresh tokens", "error", err, "userID", userID)
		return errors.Wrap(err, "failed to delete all refresh tokens")
	}

	srv.logger.Info("Successfully logged out from all devices", "userID", userID)
	return nil
}

// GetActiveSessions retrieves all active sessions for a user.
func (srv *userService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error) {
	srv.logger.Debug("Getting active sessions", "userID", userID)

	// Single query operation - use direct repository instance
	sessions, err := srv.refreshTokenRepo.FindRefreshTokensByUserID(ctx, userID)
	if err != nil {
		srv.logger.Error("Failed to get active sessions", "error", err, "userID", userID)
		return nil, errors.Wrap(err, "failed to get active sessions")
	}

	return sessions, nil
}

// RevokeSession revokes a specific session by refresh token ID.
func (srv *userService) RevokeSession(ctx context.Context, userID, tokenID uuid.UUID) error {
	srv.logger.Info("Attempting to revoke session", "userID", userID, "tokenID", tokenID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		refreshRepo := repoFactory.NewRefreshTokenRepository()

		// Verify the token belongs to the user before deleting
		token, err := refreshRepo.FindRefreshTokenByID(ctx, tokenID)
		if err != nil {
			return errors.Wrap(err, "failed to find refresh token")
		}

		if token.UserID != userID {
			return errors.Wrap(domainerrors.ErrForbidden, "token does not belong to user")
		}

		if err := refreshRepo.DeleteRefreshToken(ctx, tokenID); err != nil {
			return errors.Wrap(err, "failed to delete refresh token")
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to revoke session", "error", err, "userID", userID, "tokenID", tokenID)
		return errors.Wrap(err, "failed to revoke session")
	}

	srv.logger.Info("Successfully revoked session", "userID", userID, "tokenID", tokenID)
	return nil
}

// LinkGoogleAccount links a Google account to an existing user account.
func (srv *userService) LinkGoogleAccount(ctx context.Context, userID uuid.UUID, idToken string) error {
	srv.logger.Info("Linking Google account to existing user", "userID", userID)

	// 1. Verify the Google ID token
	oauthUser, err := srv.googleAuthService.VerifyIDToken(ctx, idToken)
	if err != nil {
		return errors.Wrap(err, "failed to verify Google ID token")
	}

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.NewAuthRepository()
		userRepo := repoFactory.NewUserRepository()

		// 1. Verify the user exists
		user, err := userRepo.FindByID(ctx, userID)
		if err != nil {
			return errors.Wrap(err, "failed to find user")
		}

		// 2. Check if Google account is already linked to another user
		existingAuth, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeGoogle, oauthUser.ID)
		if err != nil && !errors.Is(err, repository.ErrAuthNotFound) {
			return errors.Wrap(err, "failed to check existing Google authentication")
		}

		if existingAuth != nil {
			if existingAuth.UserID == userID {
				// Already linked to this user
				return errors.Wrap(domainerrors.ErrConflict, "Google account already linked to this user")
			}
			// Linked to different user
			return errors.Wrap(domainerrors.ErrConflict, "Google account already linked to another user")
		}

		// 3. Check if user already has a Google authentication method
		userGoogleAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle)
		if err != nil && !errors.Is(err, repository.ErrAuthNotFound) {
			return errors.Wrap(err, "failed to check user's Google authentication")
		}

		if userGoogleAuth != nil {
			// Update existing Google authentication
			userGoogleAuth.ProviderUserID = oauthUser.ID
			if err := authRepo.UpdateAuthentication(ctx, userGoogleAuth); err != nil {
				return errors.Wrap(err, "failed to update Google authentication")
			}
		} else {
			// Create new Google authentication
			newAuth := &entity.Authentication{
				UserID:         userID,
				Provider:       entity.ProviderTypeGoogle,
				ProviderUserID: oauthUser.ID,
			}

			if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
				return errors.Wrap(err, "failed to create Google authentication")
			}
		}

		// 4. Update user's email if it's different (optional, based on business rules)
		if user.Email != oauthUser.Email {
			srv.logger.Info("Google email differs from user email",
				"userID", userID,
				"userEmail", user.Email,
				"googleEmail", oauthUser.Email)
			// Note: In a real application, you might want to verify the email change
			// or ask the user to confirm this change
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to link Google account", "error", err, "userID", userID)
		return errors.Wrap(err, "failed to link Google account")
	}

	srv.logger.Info("Successfully linked Google account", "userID", userID)
	return nil
}

// UnlinkGoogleAccount removes the Google authentication method from a user account.
func (srv *userService) UnlinkGoogleAccount(ctx context.Context, userID uuid.UUID) error {
	srv.logger.Info("Unlinking Google account from user", "userID", userID)

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.NewAuthRepository()

		// 1. Find the user's Google authentication
		googleAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle)
		if err != nil {
			if errors.Is(err, repository.ErrAuthNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "Google account not linked to this user")
			}
			return errors.Wrap(err, "failed to find Google authentication")
		}

		// 2. Check if user has other authentication methods
		allAuths, err := authRepo.ListAuthenticationsByUserID(ctx, userID)
		if err != nil {
			return errors.Wrap(err, "failed to list user authentications")
		}

		if len(allAuths) <= 1 {
			return errors.Wrap(domainerrors.ErrValidationFailed, "cannot unlink last authentication method")
		}

		// 3. Delete the Google authentication
		if err := authRepo.DeleteAuthentication(ctx, googleAuth.ID); err != nil {
			return errors.Wrap(err, "failed to delete Google authentication")
		}

		return nil
	})

	if err != nil {
		srv.logger.Error("Failed to unlink Google account", "error", err, "userID", userID)
		return errors.Wrap(err, "failed to unlink Google account")
	}

	srv.logger.Info("Successfully unlinked Google account", "userID", userID)
	return nil
}
