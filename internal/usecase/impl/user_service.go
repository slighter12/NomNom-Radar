package impl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"radar/config"
	deliverycontext "radar/internal/delivery/context"
	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"go.uber.org/fx"
)

// userService implements the UserUsecase interface.
type userService struct {
	txManager         repository.TransactionManager
	userRepo          repository.UserRepository
	authRepo          repository.AuthRepository
	refreshTokenRepo  repository.RefreshTokenRepository
	hasher            service.PasswordHasher
	tokenService      service.TokenService
	googleAuthService service.OAuthAuthService
	maxActiveSessions int
	logger            *slog.Logger
}

type registrationConfig struct {
	Name               string
	Email              string
	Password           string
	Role               entity.Role
	BuildNewUser       func() (*entity.User, error)
	AttachProfile      func(*entity.User) error
	HasProfile         func(*entity.User) bool
	ProfileExistsError func() error
}

// UserServiceParams holds dependencies for UserService, injected by Fx.
type UserServiceParams struct {
	fx.In

	TxManager         repository.TransactionManager
	UserRepo          repository.UserRepository
	AuthRepo          repository.AuthRepository
	RefreshTokenRepo  repository.RefreshTokenRepository
	Hasher            service.PasswordHasher
	TokenService      service.TokenService
	GoogleAuthService service.OAuthAuthService
	Config            *config.Config
	Logger            *slog.Logger
}

// NewUserService is the constructor for userService. It receives all dependencies as interfaces.
func NewUserService(params UserServiceParams) usecase.UserUsecase {
	maxActiveSessions := 0
	if params.Config != nil && params.Config.Auth != nil {
		maxActiveSessions = params.Config.Auth.MaxActiveSessions
	}

	return &userService{
		txManager:         params.TxManager,
		userRepo:          params.UserRepo,
		authRepo:          params.AuthRepo,
		refreshTokenRepo:  params.RefreshTokenRepo,
		hasher:            params.Hasher,
		tokenService:      params.TokenService,
		googleAuthService: params.GoogleAuthService,
		maxActiveSessions: maxActiveSessions,
		logger:            params.Logger,
	}
}

// log returns a request-scoped logger if available, otherwise falls back to the service's logger.
func (srv *userService) log(ctx context.Context) *slog.Logger {
	return deliverycontext.GetLoggerOrDefault(ctx, srv.logger)
}

// RegisterUser orchestrates the unified user registration flow.
func (srv *userService) RegisterUser(ctx context.Context, input *usecase.RegisterUserInput) (*usecase.AuthResult, error) {
	return srv.authenticate(ctx, &authRequest{
		Method:        authMethodEmailPassword,
		Intent:        authIntentRegister,
		Provider:      entity.ProviderTypeEmail,
		RequestedRole: entity.RoleUser,
		Name:          input.Name,
		Email:         input.Email,
		Password:      input.Password,
	})
}

func (srv *userService) createUserWithProfile(ctx context.Context, cfg *registrationConfig, userRepo repository.UserRepository) (*entity.User, error) {
	newUser, err := cfg.BuildNewUser()
	if err != nil {
		return nil, err
	}
	if newUser.Name == "" {
		newUser.Name = cfg.Name
	}
	newUser.Email = cfg.Email

	if err := userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user during registration: %w", err)
	}

	return newUser, nil
}

func (srv *userService) syncExistingAccountProfile(
	ctx context.Context,
	cfg *registrationConfig,
	userRepo repository.UserRepository,
	existingUser *entity.User,
	failIfProfileExists bool,
) error {
	if cfg.HasProfile(existingUser) {
		if failIfProfileExists {
			srv.log(ctx).Warn("Profile already exists for account", slog.Any("role", cfg.Role), slog.Any("user_id", existingUser.ID))

			return cfg.ProfileExistsError()
		}

		return nil
	}

	if cfg.Name != "" {
		existingUser.Name = cfg.Name
	}

	if err := cfg.AttachProfile(existingUser); err != nil {
		return err
	}

	if err := userRepo.Update(ctx, existingUser); err != nil {
		return fmt.Errorf("failed to update user profile during registration: %w", err)
	}

	return nil
}

func buildNewUserEntity(name, email string) *entity.User {
	return &entity.User{
		Name:        name,
		Email:       email,
		UserProfile: &entity.UserProfile{},
	}
}

type merchantProfileSeed struct {
	StoreName       string
	BusinessLicense string
}

func attachUserProfile(user *entity.User) {
	user.UserProfile = &entity.UserProfile{UserID: user.ID}
}

func buildNewMerchantEntity(input *usecase.RegisterMerchantInput) (*entity.User, error) {
	return buildNewMerchantEntityFromSeed(input.Name, input.Email, merchantProfileSeed{
		StoreName:       input.StoreName,
		BusinessLicense: input.BusinessLicense,
	})
}

func buildNewMerchantEntityFromSeed(name, email string, seed merchantProfileSeed) (*entity.User, error) {
	merchantProfile, err := buildMerchantProfile(seed, uuid.Nil)
	if err != nil {
		return nil, err
	}

	return &entity.User{
		Name:            name,
		Email:           email,
		MerchantProfile: merchantProfile,
	}, nil
}

func attachMerchantProfile(input *usecase.RegisterMerchantInput) func(*entity.User) error {
	return attachMerchantProfileFromSeed(merchantProfileSeed{
		StoreName:       input.StoreName,
		BusinessLicense: input.BusinessLicense,
	})
}

func attachMerchantProfileFromSeed(seed merchantProfileSeed) func(*entity.User) error {
	return func(user *entity.User) error {
		merchantProfile, err := buildMerchantProfile(seed, user.ID)
		if err != nil {
			return err
		}

		user.MerchantProfile = merchantProfile

		return nil
	}
}

func userHasUserProfile(user *entity.User) bool {
	return user.UserProfile != nil
}

func userHasMerchantProfile(user *entity.User) bool {
	return user.MerchantProfile != nil
}

func maskEmailForLog(email string) string {
	atIndex := strings.Index(email, "@")
	if atIndex <= 0 || atIndex == len(email)-1 {
		return "***"
	}

	if atIndex <= 3 {
		return "***" + email[atIndex:]
	}

	return email[:3] + "***" + email[atIndex:]
}

// RegisterMerchant orchestrates the unified merchant registration flow.
func (srv *userService) RegisterMerchant(ctx context.Context, input *usecase.RegisterMerchantInput) (*usecase.AuthResult, error) {
	return srv.authenticate(ctx, &authRequest{
		Method:        authMethodEmailPassword,
		Intent:        authIntentRegister,
		Provider:      entity.ProviderTypeEmail,
		RequestedRole: entity.RoleMerchant,
		Name:          input.Name,
		Email:         input.Email,
		Password:      input.Password,
		MerchantSeed: &merchantProfileSeed{
			StoreName:       input.StoreName,
			BusinessLicense: input.BusinessLicense,
		},
	})
}

// Login orchestrates the unified email login flow.
func (srv *userService) Login(ctx context.Context, input *usecase.LoginInput) (*usecase.AuthResult, error) {
	return srv.authenticate(ctx, &authRequest{
		Method:   authMethodEmailPassword,
		Intent:   authIntentLogin,
		Provider: entity.ProviderTypeEmail,
		Email:    input.Email,
		Password: input.Password,
	})
}

func (srv *userService) persistLoginRefreshToken(ctx context.Context, userID uuid.UUID, refreshTokenString string) error {
	if srv.maxActiveSessions > 0 {
		// When session limit is enabled, keep lock/count/insert in one short transaction.
		if err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
			return srv.storeRefreshToken(ctx, repoFactory, userID, refreshTokenString)
		}); err != nil {
			return fmt.Errorf("failed to execute user login transaction: %w", err)
		}

		return nil
	}

	// No session limit: direct insert avoids unnecessary transaction overhead.
	if err := srv.storeRefreshTokenDirect(ctx, userID, refreshTokenString); err != nil {
		return fmt.Errorf("failed to create refresh token during login: %w", err)
	}

	return nil
}

// RefreshToken handles the process of issuing a new access token using a refresh token.
// The refresh token remains unchanged for security reasons.
func (srv *userService) RefreshToken(ctx context.Context, input *usecase.RefreshTokenInput) (*usecase.RefreshTokenOutput, error) {
	srv.log(ctx).Info("Attempting to refresh access token")

	claims, err := srv.tokenService.ValidateToken(input.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}
	if claims.Type != service.TokenTypeRefresh {
		return nil, fmt.Errorf("invalid token type: %w", domainerrors.ErrUnauthorized)
	}

	var newAccessToken string

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		refreshRepo := repoFactory.RefreshTokenRepo()

		// 1. Verify the refresh token exists in the database.
		tokenHash := srv.tokenService.HashToken(input.RefreshToken)

		_, err := refreshRepo.FindRefreshTokenByHash(ctx, tokenHash)
		if err != nil {
			return fmt.Errorf("refresh token not found or expired: %w", err)
		}

		// 2. Fetch user and roles.
		user, err := userRepo.FindByID(ctx, claims.UserID)
		if err != nil {
			return fmt.Errorf("failed to find user: %w", err)
		}
		var roles entity.Roles
		if user.UserProfile != nil {
			roles = append(roles, entity.RoleUser)
		}
		if user.MerchantProfile != nil {
			roles = append(roles, entity.RoleMerchant)
		}

		// 3. Generate only a new access token (refresh token remains unchanged).
		newAccessToken, _, err = srv.tokenService.GenerateTokens(user.ID, roles.ToStrings())
		if err != nil {
			return fmt.Errorf("failed to generate new access token: %w", err)
		}

		// Note: We don't modify the refresh token - it remains valid and unchanged
		// This prevents token rotation attacks and maintains better security

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to execute refresh token transaction", slog.Any("error", err))

		return nil, fmt.Errorf("failed to execute refresh token transaction: %w", err)
	}

	return &usecase.RefreshTokenOutput{
		AccessToken: newAccessToken,
	}, nil
}

// Logout handles the process of invalidating a user's session by deleting their refresh token.
func (srv *userService) Logout(ctx context.Context, input *usecase.LogoutInput) error {
	srv.log(ctx).Info("Attempting to log out")

	_, err := srv.tokenService.ValidateToken(input.RefreshToken)
	if err != nil {
		// Even if the token is invalid, we can proceed to delete it from the database.
		srv.log(ctx).Warn("Logout with invalid token", slog.Any("error", err))
	}

	tokenHash := srv.tokenService.HashToken(input.RefreshToken)

	// Single operation - use direct repository instance
	if err := srv.refreshTokenRepo.DeleteRefreshTokenByHash(ctx, tokenHash); err != nil {
		srv.log(ctx).Error("Failed to delete refresh token", slog.Any("error", err))

		return fmt.Errorf("failed to delete refresh token: %w", err)
	}
	srv.log(ctx).Info("Successfully logged out")

	return nil
}

// GoogleCallback handles Google sign-in via the unified auth flow.
func (srv *userService) GoogleCallback(ctx context.Context, input *usecase.GoogleCallbackInput) (*usecase.AuthResult, error) {
	requestedRole := normalizeRequestedRole(input.RequestedRole, input.State)

	var merchantSeed *merchantProfileSeed
	if strings.TrimSpace(input.StoreName) != "" || strings.TrimSpace(input.BusinessLicense) != "" {
		merchantSeed = &merchantProfileSeed{
			StoreName:       input.StoreName,
			BusinessLicense: input.BusinessLicense,
		}
	}

	return srv.authenticate(ctx, &authRequest{
		Method:        authMethodOAuth,
		Intent:        authIntentLogin,
		RequestedRole: requestedRole,
		IDToken:       input.IDToken,
		MerchantSeed:  merchantSeed,
	})
}

func normalizeRequestedRole(requestedRole, legacyState string) entity.Role {
	role := strings.TrimSpace(strings.ToLower(requestedRole))
	if role == "" {
		role = strings.TrimSpace(strings.ToLower(legacyState))
	}

	switch role {
	case entity.RoleMerchant.String():
		return entity.RoleMerchant
	case entity.RoleUser.String():
		return entity.RoleUser
	default:
		return entity.RoleUser
	}
}

func buildMerchantProfile(seed merchantProfileSeed, userID uuid.UUID) (*entity.MerchantProfile, error) {
	storeName := strings.TrimSpace(seed.StoreName)
	if storeName == "" {
		return nil, fmt.Errorf("store_name is required for merchant sign-in: %w", domainerrors.ErrValidationFailed)
	}

	businessLicense := strings.TrimSpace(seed.BusinessLicense)
	if businessLicense == "" {
		return nil, fmt.Errorf("business_license is required for merchant sign-in: %w", domainerrors.ErrValidationFailed)
	}

	profile := &entity.MerchantProfile{
		StoreName:       storeName,
		BusinessLicense: businessLicense,
	}
	if userID != uuid.Nil {
		profile.UserID = userID
	}

	return profile, nil
}

func createOAuthAuthentication(ctx context.Context, authRepo repository.AuthRepository, userID uuid.UUID, provider entity.ProviderType, providerUserID string) error {
	newAuth := &entity.Authentication{
		UserID:         userID,
		Provider:       provider,
		ProviderUserID: providerUserID,
	}

	if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
		return fmt.Errorf("failed to create OAuth authentication: %w", err)
	}

	return nil
}

// extractUserRoles extracts roles from user profiles
func (srv *userService) extractUserRoles(user *entity.User) entity.Roles {
	var roles entity.Roles

	if user.UserProfile != nil {
		roles = append(roles, entity.RoleUser)
	}
	if user.MerchantProfile != nil {
		roles = append(roles, entity.RoleMerchant)
	}

	return roles
}

// storeRefreshToken stores the refresh token in the database
func (srv *userService) storeRefreshToken(ctx context.Context, repoFactory repository.RepositoryFactory, userID uuid.UUID, refreshTokenString string) error {
	refreshRepo := repoFactory.RefreshTokenRepo()
	userRepo := repoFactory.UserRepo()

	// Defensive: check maxActiveSessions here because storeRefreshToken is called
	// from multiple sites (e.g. handleOAuthUserAuth), not only persistLoginRefreshToken.
	if srv.maxActiveSessions > 0 {
		if err := userRepo.AcquireSessionMutex(ctx, userID); err != nil {
			return fmt.Errorf("failed to lock user row for session limit check: %w", err)
		}

		activeSessions, err := refreshRepo.CountActiveSessionsByUserID(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to count active sessions: %w", err)
		}
		if activeSessions >= srv.maxActiveSessions {
			return fmt.Errorf("active session limit exceeded: %w", domainerrors.ErrSessionLimitExceeded)
		}
	}

	return srv.storeRefreshTokenWithRepo(ctx, refreshRepo, userID, refreshTokenString)
}

func (srv *userService) storeRefreshTokenDirect(ctx context.Context, userID uuid.UUID, refreshTokenString string) error {
	return srv.storeRefreshTokenWithRepo(ctx, srv.refreshTokenRepo, userID, refreshTokenString)
}

func (srv *userService) storeRefreshTokenWithRepo(ctx context.Context, refreshRepo repository.RefreshTokenRepository, userID uuid.UUID, refreshTokenString string) error {
	// Hash the refresh token
	refreshTokenHash := srv.tokenService.HashToken(refreshTokenString)

	newRefreshToken := &entity.RefreshToken{
		UserID:    userID,
		TokenHash: refreshTokenHash,
		ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
	}

	if err := refreshRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}

	return nil
}

// LogoutAllDevices handles the process of invalidating all user sessions by deleting all refresh tokens.
func (srv *userService) LogoutAllDevices(ctx context.Context, userID uuid.UUID) error {
	srv.log(ctx).Info("Attempting to log out from all devices", slog.Any("user_id", userID))

	// Single operation - use direct repository instance
	if err := srv.refreshTokenRepo.DeleteRefreshTokensByUserID(ctx, userID); err != nil {
		srv.log(ctx).Error("Failed to delete all refresh tokens", slog.Any("error", err), slog.Any("user_id", userID))

		return fmt.Errorf("failed to delete all refresh tokens: %w", err)
	}
	srv.log(ctx).Info("Successfully logged out from all devices", slog.Any("user_id", userID))

	return nil
}

// GetActiveSessions retrieves all active sessions for a user.
func (srv *userService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error) {
	srv.log(ctx).Debug("Getting active sessions", slog.Any("user_id", userID))

	// Single query operation - use direct repository instance
	sessions, err := srv.refreshTokenRepo.FindRefreshTokensByUserID(ctx, userID)
	if err != nil {
		srv.log(ctx).Error("Failed to get active sessions", slog.Any("error", err), slog.Any("user_id", userID))

		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	return sessions, nil
}

// RevokeSession revokes a specific session by refresh token ID.
func (srv *userService) RevokeSession(ctx context.Context, userID, tokenID uuid.UUID) error {
	srv.log(ctx).Info("Attempting to revoke session", slog.Any("user_id", userID), slog.Any("token_id", tokenID))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		refreshRepo := repoFactory.RefreshTokenRepo()

		// Verify the token belongs to the user before deleting
		token, err := refreshRepo.FindRefreshTokenByID(ctx, tokenID)
		if err != nil {
			return fmt.Errorf("failed to find refresh token: %w", err)
		}

		if token.UserID != userID {
			return fmt.Errorf("token does not belong to user: %w", domainerrors.ErrForbidden)
		}

		if err := refreshRepo.DeleteRefreshToken(ctx, tokenID); err != nil {
			return fmt.Errorf("failed to delete refresh token: %w", err)
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to revoke session", slog.Any("error", err), slog.Any("user_id", userID), slog.Any("token_id", tokenID))

		return fmt.Errorf("failed to revoke session: %w", err)
	}
	srv.log(ctx).Info("Successfully revoked session", slog.Any("user_id", userID), slog.Any("token_id", tokenID))

	return nil
}

// LinkGoogleAccount links a Google account to an existing user account.
func (srv *userService) LinkGoogleAccount(ctx context.Context, userID uuid.UUID, idToken string) error {
	srv.log(ctx).Info("Linking Google account to existing user", slog.Any("user_id", userID))

	// 1. Verify the Google ID token
	oauthUser, err := srv.googleAuthService.VerifyIDToken(ctx, idToken)
	if err != nil {
		return fmt.Errorf("failed to verify Google ID token: %w", err)
	}

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		return srv.performGoogleAccountLinking(ctx, repoFactory, userID, oauthUser)
	})

	if err != nil {
		srv.log(ctx).Error("Failed to link Google account", slog.Any("error", err), slog.Any("user_id", userID))

		return fmt.Errorf("failed to link Google account: %w", err)
	}
	srv.log(ctx).Info("Successfully linked Google account", slog.Any("user_id", userID))

	return nil
}

// performGoogleAccountLinking handles the core logic for linking a Google account
func (srv *userService) performGoogleAccountLinking(ctx context.Context, repoFactory repository.RepositoryFactory, userID uuid.UUID, oauthUser *service.OAuthUser) error {
	authRepo := repoFactory.AuthRepo()
	userRepo := repoFactory.UserRepo()

	// 1. Verify the user exists
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// 2. Check for conflicts with other users
	if err := srv.checkGoogleAccountConflicts(ctx, authRepo, userID, oauthUser.ID); err != nil {
		return err
	}

	// 3. Create or update Google authentication
	if err := srv.createOrUpdateGoogleAuth(ctx, authRepo, userID, oauthUser.ID); err != nil {
		return err
	}

	// Note: In a real application, you might want to verify the email change
	// or ask the user to confirm this change
	if user.Email != oauthUser.Email {
		srv.log(ctx).Info("Google email differs from user email",
			slog.Any("user_id", userID),
			slog.String("user_email", user.Email),
			slog.String("google_email", oauthUser.Email))
	}

	return nil
}

// checkGoogleAccountConflicts checks if the Google account is already linked to another user
func (srv *userService) checkGoogleAccountConflicts(ctx context.Context, authRepo repository.AuthRepository, userID uuid.UUID, googleUserID string) error {
	existingAuth, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeGoogle, googleUserID)
	if err != nil && !errors.Is(err, repository.ErrAuthNotFound) {
		return fmt.Errorf("failed to check existing Google authentication: %w", err)
	}

	if existingAuth != nil {
		if existingAuth.UserID == userID {
			return fmt.Errorf("google account already linked to this user: %w", domainerrors.ErrConflict)
		}

		return fmt.Errorf("google account already linked to another user: %w", domainerrors.ErrConflict)
	}

	return nil
}

// createOrUpdateGoogleAuth creates or updates the Google authentication for the user
func (srv *userService) createOrUpdateGoogleAuth(ctx context.Context, authRepo repository.AuthRepository, userID uuid.UUID, googleUserID string) error {
	userGoogleAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle)
	if err != nil && !errors.Is(err, repository.ErrAuthNotFound) {
		return fmt.Errorf("failed to check user's Google authentication: %w", err)
	}

	if userGoogleAuth != nil {
		// Update existing Google authentication
		userGoogleAuth.ProviderUserID = googleUserID
		if err := authRepo.UpdateAuthentication(ctx, userGoogleAuth); err != nil {
			return fmt.Errorf("failed to update Google authentication: %w", err)
		}
	} else {
		// Create new Google authentication
		newAuth := &entity.Authentication{
			UserID:         userID,
			Provider:       entity.ProviderTypeGoogle,
			ProviderUserID: googleUserID,
		}

		if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
			return fmt.Errorf("failed to create Google authentication: %w", err)
		}
	}

	return nil
}

// UnlinkGoogleAccount removes the Google authentication method from a user account.
func (srv *userService) UnlinkGoogleAccount(ctx context.Context, userID uuid.UUID) error {
	srv.log(ctx).Info("Unlinking Google account from user", slog.Any("user_id", userID))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.AuthRepo()

		// 1. Find the user's Google authentication
		googleAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle)
		if err != nil {
			if errors.Is(err, repository.ErrAuthNotFound) {
				return fmt.Errorf("google account not linked to this user: %w", domainerrors.ErrNotFound)
			}

			return fmt.Errorf("failed to find Google authentication: %w", err)
		}

		// 2. Check if user has other authentication methods
		allAuths, err := authRepo.ListAuthenticationsByUserID(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to list user authentications: %w", err)
		}

		if len(allAuths) <= 1 {
			return fmt.Errorf("cannot unlink last authentication method: %w", domainerrors.ErrValidationFailed)
		}

		// 3. Delete the Google authentication
		if err := authRepo.DeleteAuthentication(ctx, googleAuth.ID); err != nil {
			return fmt.Errorf("failed to delete Google authentication: %w", err)
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to unlink Google account", slog.Any("error", err), slog.Any("user_id", userID))

		return fmt.Errorf("failed to unlink Google account: %w", err)
	}
	srv.log(ctx).Info("Successfully unlinked Google account", slog.Any("user_id", userID))

	return nil
}
