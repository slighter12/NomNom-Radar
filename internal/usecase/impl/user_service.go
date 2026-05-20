package impl

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"radar/config"
	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/policy"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/platform/observability"
	"radar/internal/usecase"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

// userService implements the UserUsecase interface.
type userService struct {
	txManager           repository.TransactionManager
	userRepo            repository.UserRepository
	authRepo            repository.AuthRepository
	refreshTokenRepo    repository.RefreshTokenRepository
	loginAttemptRepo    repository.LoginAttemptRepository
	deviceRepo          repository.DeviceRepository
	hasher              service.PasswordHasher
	tokenService        service.TokenService
	googleAuthService   service.OAuthAuthService
	notificationSvc     service.NotificationService
	maxActiveSessions   int
	loginThrottleCfg    config.LoginThrottleConfig
	loginThrottlePolicy policy.LoginThrottlePolicy
	notificationTimeout time.Duration
	logger              *slog.Logger
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
	LoginAttemptRepo  repository.LoginAttemptRepository
	DeviceRepo        repository.DeviceRepository
	Hasher            service.PasswordHasher
	TokenService      service.TokenService
	GoogleAuthService service.OAuthAuthService
	NotificationSvc   service.NotificationService
	Config            *config.Config
	Logger            *slog.Logger
}

// NewUserService is the constructor for userService. It receives all dependencies as interfaces.
func NewUserService(params UserServiceParams) usecase.UserUsecase {
	cfg := params.Config
	if cfg == nil {
		cfg = &config.Config{}
	}
	config.ApplyDefaults(cfg)

	maxActiveSessions := cfg.Auth.MaxActiveSessions
	loginThrottleCfg := *cfg.LoginThrottle
	notificationTimeout := cfg.Notification.Timeout

	return &userService{
		txManager:           params.TxManager,
		userRepo:            params.UserRepo,
		authRepo:            params.AuthRepo,
		refreshTokenRepo:    params.RefreshTokenRepo,
		loginAttemptRepo:    params.LoginAttemptRepo,
		deviceRepo:          params.DeviceRepo,
		hasher:              params.Hasher,
		tokenService:        params.TokenService,
		googleAuthService:   params.GoogleAuthService,
		notificationSvc:     params.NotificationSvc,
		maxActiveSessions:   maxActiveSessions,
		loginThrottleCfg:    loginThrottleCfg,
		loginThrottlePolicy: policy.DefaultLoginThrottlePolicy(),
		notificationTimeout: notificationTimeout,
		logger:              params.Logger,
	}
}

// log returns a request-scoped logger if available, otherwise falls back to the service's logger.
func (srv *userService) log(ctx context.Context) *slog.Logger {
	return observability.LoggerFromContextOrDefault(ctx, srv.logger)
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
			srv.log(ctx).Warn("Profile already exists for account", slog.String("role", cfg.Role.String()), slog.String("user_id", existingUser.ID.String()))

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
	StoreName string
}

func attachUserProfile(user *entity.User) {
	user.UserProfile = &entity.UserProfile{UserID: user.ID}
}

func buildNewMerchantEntity(input *usecase.RegisterMerchantInput) (*entity.User, error) {
	return buildNewMerchantEntityFromSeed(input.Name, input.Email, merchantProfileSeed{
		StoreName: input.StoreName,
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
		StoreName: input.StoreName,
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
			StoreName: input.StoreName,
		},
	})
}

// Login orchestrates the unified email login flow.
func (srv *userService) Login(ctx context.Context, input *usecase.LoginInput) (*usecase.AuthResult, error) {
	attempt, err := srv.checkLoginThrottle(ctx, input.Email)
	if err != nil {
		return nil, err
	}

	output, err := srv.authenticate(ctx, &authRequest{
		Method:   authMethodEmailPassword,
		Intent:   authIntentLogin,
		Provider: entity.ProviderTypeEmail,
		Email:    input.Email,
		Password: input.Password,
	})
	if err != nil {
		if errors.Is(err, domainerrors.ErrInvalidCredentials) {
			if recordErr := srv.recordLoginFailure(ctx, input.Email, attempt.UserID); recordErr != nil {
				return nil, recordErr
			}
		}

		return nil, err
	}

	if err := srv.recordLoginSuccess(ctx, input.Email); err != nil {
		return nil, err
	}

	return output, nil
}

func (srv *userService) persistLoginRefreshToken(ctx context.Context, userID uuid.UUID, refreshTokenString string) error {
	familyID := uuid.New()

	if srv.maxActiveSessions > 0 {
		// When session limit is enabled, keep lock/count/insert in one short transaction.
		if err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
			return srv.storeRefreshToken(ctx, repoFactory, userID, refreshTokenString, familyID)
		}); err != nil {
			return err
		}

		return nil
	}

	// No session limit: direct insert avoids unnecessary transaction overhead.
	if err := srv.storeRefreshTokenDirect(ctx, userID, refreshTokenString, familyID); err != nil {
		return err
	}

	return nil
}

type refreshTokenRotationResult struct {
	AccessToken   string
	RefreshToken  string
	ReuseDetected bool
}

// RefreshToken handles refresh token rotation with token family reuse detection.
func (srv *userService) RefreshToken(ctx context.Context, input *usecase.RefreshTokenInput) (*usecase.RefreshTokenOutput, error) {
	srv.log(ctx).Info("Attempting to refresh access token")

	claims, err := srv.validateRefreshTokenInput(input.RefreshToken)
	if err != nil {
		return nil, err
	}

	tokenHash := srv.tokenService.HashToken(input.RefreshToken)
	result, err := srv.rotateRefreshTokenPair(ctx, claims, tokenHash)

	if err != nil {
		if _, ok := errors.AsType[domainerrors.AppError](err); !ok {
			srv.log(ctx).Error("Failed to execute refresh token transaction", slog.String("error", err.Error()))
		}

		return nil, err
	}
	if result.ReuseDetected {
		srv.sendTokenReuseNotification(ctx, claims.UserID)

		return nil, domainerrors.ErrRefreshTokenInvalid
	}

	return &usecase.RefreshTokenOutput{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}, nil
}

func (srv *userService) validateRefreshTokenInput(refreshToken string) (*service.Claims, error) {
	claims, err := srv.tokenService.ValidateToken(refreshToken)
	if err == nil {
		if claims.Type != service.TokenTypeRefresh {
			return nil, domainerrors.ErrUnauthorized
		}

		return claims, nil
	}

	if !errors.Is(err, jwt.ErrTokenExpired) {
		return nil, domainerrors.ErrRefreshTokenInvalid
	}

	token, _, parseErr := new(jwt.Parser).ParseUnverified(refreshToken, &service.Claims{})
	if parseErr != nil {
		return nil, domainerrors.ErrRefreshTokenInvalid
	}

	expiredClaims, ok := token.Claims.(*service.Claims)
	if !ok {
		return nil, domainerrors.ErrRefreshTokenInvalid
	}

	if expiredClaims.Type != service.TokenTypeRefresh {
		return nil, domainerrors.ErrUnauthorized
	}

	return expiredClaims, nil
}

func (srv *userService) rotateRefreshTokenPair(
	ctx context.Context,
	claims *service.Claims,
	tokenHash string,
) (*refreshTokenRotationResult, error) {
	result := &refreshTokenRotationResult{}

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		return srv.executeRefreshTokenRotationTx(ctx, repoFactory, claims, tokenHash, result)
	})
	if err != nil {
		return nil, mapRefreshTokenRotationError(err)
	}

	return result, nil
}

func mapRefreshTokenRotationError(err error) error {
	if appErr, ok := errors.AsType[domainerrors.AppError](err); ok {
		return appErr
	}

	return fmt.Errorf("execute refresh token transaction: %w", err)
}

func (srv *userService) executeRefreshTokenRotationTx(
	ctx context.Context,
	repoFactory repository.RepositoryFactory,
	claims *service.Claims,
	tokenHash string,
	result *refreshTokenRotationResult,
) error {
	refreshRepo := repoFactory.RefreshTokenRepo()
	userRepo := repoFactory.UserRepo()

	if err := userRepo.AcquireSessionMutex(ctx, claims.UserID); err != nil {
		return err
	}

	storedToken, err := loadStoredRefreshToken(ctx, refreshRepo, tokenHash)
	if err != nil {
		return err
	}
	if storedToken.UserID != claims.UserID {
		return domainerrors.ErrRefreshTokenInvalid
	}

	if storedToken.IsRevoked {
		return handleRevokedRefreshToken(ctx, refreshRepo, storedToken, result)
	}
	if storedToken.ExpiresAt.Before(time.Now()) {
		return domainerrors.ErrRefreshTokenExpired
	}

	return srv.rotateActiveRefreshToken(ctx, refreshRepo, userRepo, storedToken, result)
}

func handleRevokedRefreshToken(
	ctx context.Context,
	refreshRepo repository.RefreshTokenRepository,
	storedToken *entity.RefreshToken,
	result *refreshTokenRotationResult,
) error {
	if err := markRefreshTokenReuse(ctx, refreshRepo, storedToken.FamilyID); err != nil {
		return err
	}

	result.ReuseDetected = true

	return nil
}

func (srv *userService) rotateActiveRefreshToken(
	ctx context.Context,
	refreshRepo repository.RefreshTokenRepository,
	userRepo repository.UserRepository,
	storedToken *entity.RefreshToken,
	result *refreshTokenRotationResult,
) error {
	user, _, err := srv.loadRefreshTokenUser(ctx, userRepo, storedToken.UserID)
	if err != nil {
		return err
	}

	accessToken, refreshToken, err := srv.issueAndStoreRotatedTokenPair(ctx, refreshRepo, user, storedToken)
	if err != nil {
		return err
	}

	result.AccessToken = accessToken
	result.RefreshToken = refreshToken

	return nil
}

func (srv *userService) issueAndStoreRotatedTokenPair(
	ctx context.Context,
	refreshRepo repository.RefreshTokenRepository,
	user *entity.User,
	storedToken *entity.RefreshToken,
) (string, string, error) {
	accessToken, refreshToken, refreshTokenHash, err := srv.tokenService.RotateTokens(
		user.ID,
		srv.extractUserRoles(user).ToStrings(),
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to rotate refresh token pair: %w", err)
	}

	newRefreshToken := &entity.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshTokenHash,
		FamilyID:  storedToken.FamilyID,
		ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
	}
	if err := refreshRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
		return "", "", err
	}

	storedToken.IsRevoked = true
	storedToken.ReplacedBy = &newRefreshToken.ID
	if err := refreshRepo.UpdateRefreshToken(ctx, storedToken); err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func loadStoredRefreshToken(
	ctx context.Context,
	refreshRepo repository.RefreshTokenRepository,
	tokenHash string,
) (*entity.RefreshToken, error) {
	storedToken, err := refreshRepo.FindRefreshTokenByHashIncludingRevoked(ctx, tokenHash)
	if errors.Is(err, domainerrors.ErrRefreshTokenNotFound) {
		return nil, domainerrors.ErrRefreshTokenInvalid
	}
	if err != nil {
		return nil, err
	}

	return storedToken, nil
}

func markRefreshTokenReuse(
	ctx context.Context,
	refreshRepo repository.RefreshTokenRepository,
	familyID uuid.UUID,
) error {
	return refreshRepo.RevokeTokenFamily(ctx, familyID)
}

func (srv *userService) loadRefreshTokenUser(
	ctx context.Context,
	userRepo repository.UserRepository,
	userID uuid.UUID,
) (*entity.User, bool, error) {
	user, err := userRepo.FindByID(ctx, userID)
	if err == nil {
		return user, false, nil
	}

	if errors.Is(err, domainerrors.ErrUserNotFound) {
		return nil, true, domainerrors.ErrUnauthorized
	}

	return nil, false, err
}

// Logout handles the process of invalidating a user's token family.
func (srv *userService) Logout(ctx context.Context, input *usecase.LogoutInput) error {
	srv.log(ctx).Info("Attempting to log out")

	_, err := srv.tokenService.ValidateToken(input.RefreshToken)
	if err != nil {
		// Even if the token is invalid, we can proceed to delete it from the database.
		srv.log(ctx).Warn("Logout with invalid token", slog.String("error", err.Error()))
	}

	tokenHash := srv.tokenService.HashToken(input.RefreshToken)

	if err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		refreshRepo := repoFactory.RefreshTokenRepo()
		userRepo := repoFactory.UserRepo()

		token, err := refreshRepo.FindRefreshTokenByHashIncludingRevoked(ctx, tokenHash)
		if err != nil {
			if errors.Is(err, domainerrors.ErrRefreshTokenNotFound) {
				return nil
			}

			return err
		}

		if err := userRepo.AcquireSessionMutex(ctx, token.UserID); err != nil {
			return err
		}

		return refreshRepo.RevokeTokenFamily(ctx, token.FamilyID)
	}); err != nil {
		srv.log(ctx).Error("Failed to revoke refresh token family during logout", slog.String("error", err.Error()))

		return domainerrors.ErrInternalError
	}
	srv.log(ctx).Info("Successfully logged out")

	return nil
}

// GoogleCallback handles Google sign-in via the unified auth flow.
func (srv *userService) GoogleCallback(ctx context.Context, input *usecase.GoogleCallbackInput) (*usecase.AuthResult, error) {
	requestedRole := normalizeRequestedRole(input.RequestedRole, input.State)

	var merchantSeed *merchantProfileSeed
	if strings.TrimSpace(input.StoreName) != "" {
		merchantSeed = &merchantProfileSeed{
			StoreName: input.StoreName,
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

func (srv *userService) LinkProvider(ctx context.Context, input usecase.LinkProviderInput) (*usecase.LinkProviderOutput, error) {
	claims, provider, providerUserID, err := srv.parseLinkingClaims(input.LinkingToken)
	if err != nil {
		return nil, err
	}

	linkRequest := buildAuthRequestFromLinkingClaims(claims)
	resolution, err := srv.resolveLinkProviderInput(ctx, claims.UserID, provider, providerUserID)
	if err != nil {
		return nil, err
	}

	if _, err := srv.checkLoginThrottle(ctx, resolution.user.Email); err != nil {
		return nil, err
	}

	if !srv.isLinkProviderPasswordValid(input.Password, resolution.emailAuth.PasswordHash) {
		if err := srv.recordLoginFailure(ctx, resolution.user.Email, &resolution.user.ID); err != nil {
			return nil, err
		}

		return nil, domainerrors.ErrInvalidCredentials
	}

	if err := srv.recordLoginSuccess(ctx, resolution.user.Email); err != nil {
		return nil, err
	}

	output, err := srv.executeProviderLinking(ctx, linkRequest, resolution, provider, providerUserID)
	if err != nil {
		return nil, err
	}
	if output != nil {
		return output, nil
	}

	return srv.buildAuthenticatedResult(ctx, resolution.user)
}

type linkProviderResolution struct {
	user      *entity.User
	emailAuth *entity.Authentication
}

func (srv *userService) parseLinkingClaims(linkingToken string) (*service.Claims, entity.ProviderType, string, error) {
	claims, err := srv.tokenService.ValidateToken(linkingToken)
	if err != nil || claims.Type != service.TokenTypeLinking {
		return nil, "", "", domainerrors.ErrInvalidLinkingToken
	}

	provider := entity.ProviderType(strings.TrimSpace(strings.ToLower(claims.Provider)))
	providerUserID := strings.TrimSpace(claims.ProviderUserID)
	if !provider.IsOAuthProvider() || providerUserID == "" {
		return nil, "", "", domainerrors.ErrInvalidLinkingToken
	}

	return claims, provider, providerUserID, nil
}

func (srv *userService) resolveLinkProviderInput(
	ctx context.Context,
	userID uuid.UUID,
	provider entity.ProviderType,
	providerUserID string,
) (*linkProviderResolution, error) {
	resolution := &linkProviderResolution{}

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()
		authRepo := repoFactory.AuthRepo()

		user, err := userRepo.FindByID(ctx, userID)
		if errors.Is(err, domainerrors.ErrUserNotFound) {
			return domainerrors.ErrUnauthorized
		}
		if err != nil {
			return err
		}

		emailAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeEmail)
		if errors.Is(err, domainerrors.ErrAuthNotFound) {
			return domainerrors.ErrInvalidCredentials
		}
		if err != nil {
			return err
		}

		if err := validateProviderLinkAvailability(ctx, authRepo, provider, providerUserID, user.ID); err != nil {
			return err
		}

		resolution.user = user
		resolution.emailAuth = emailAuth

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resolution, nil
}

func validateProviderLinkAvailability(
	ctx context.Context,
	authRepo repository.AuthRepository,
	provider entity.ProviderType,
	providerUserID string,
	userID uuid.UUID,
) error {
	existingAuth, err := authRepo.FindAuthentication(ctx, provider, providerUserID)
	if errors.Is(err, domainerrors.ErrAuthNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if existingAuth != nil && existingAuth.UserID != userID {
		return domainerrors.ErrProviderAlreadyLinked
	}

	return nil
}

func (srv *userService) isLinkProviderPasswordValid(password, passwordHash string) bool {
	if passwordHash == "" {
		return false
	}

	return srv.hasher.Check(password, passwordHash)
}

func (srv *userService) executeProviderLinking(
	ctx context.Context,
	linkRequest *authRequest,
	resolution *linkProviderResolution,
	provider entity.ProviderType,
	providerUserID string,
) (*usecase.AuthResult, error) {
	var output *usecase.AuthResult

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		linkedResolution, err := srv.linkProviderAndSyncAccountRoles(
			ctx,
			repoFactory,
			linkRequest,
			resolution.user,
			provider,
			providerUserID,
		)
		if err != nil {
			return err
		}

		resolution.user = linkedResolution.User
		if !linkedResolution.OnboardingRequired {
			return nil
		}

		output, err = srv.buildOnboardingRequiredResult(linkedResolution.User, linkRequest.RequestedRole)

		return err
	})
	if err != nil {
		return nil, err
	}

	return output, nil
}

func buildAuthRequestFromLinkingClaims(claims *service.Claims) *authRequest {
	req := &authRequest{
		Method:        authMethodOAuth,
		Intent:        authIntentLogin,
		RequestedRole: normalizeRequestedRole(claims.RequestedRole, ""),
	}

	storeName := strings.TrimSpace(claims.StoreName)
	if storeName != "" {
		req.MerchantSeed = &merchantProfileSeed{
			StoreName: storeName,
		}
	}

	return req
}

func (srv *userService) linkProviderAndSyncAccountRoles(
	ctx context.Context,
	repoFactory repository.RepositoryFactory,
	req *authRequest,
	user *entity.User,
	provider entity.ProviderType,
	providerUserID string,
) (*authResolution, error) {
	authRepo := repoFactory.AuthRepo()
	userRepo := repoFactory.UserRepo()

	existingAuth, err := authRepo.FindAuthentication(ctx, provider, providerUserID)
	if err != nil && !errors.Is(err, domainerrors.ErrAuthNotFound) {
		return nil, err
	}
	if existingAuth != nil {
		if existingAuth.UserID != user.ID {
			return nil, domainerrors.ErrProviderAlreadyLinked
		}
	} else if err := ensureOAuthAuthLink(ctx, authRepo, user.ID, provider, providerUserID); err != nil {
		return nil, err
	}

	identity := &verifiedIdentity{
		Provider:       provider,
		ProviderUserID: providerUserID,
		Email:          user.Email,
		Name:           user.Name,
		EmailVerified:  true,
	}

	// Product design depends on account role expansion being bidirectional:
	// user -> merchant and merchant -> user must stay on this shared path for both
	// normal OAuth auth and the re-authenticated linking flow. Do not split these
	// branches unless the product account model changes.
	onboardingRequired, err := srv.ensureRequestedRole(ctx, userRepo, user, req, identity)
	if err != nil {
		return nil, err
	}

	return &authResolution{
		User:               user,
		OnboardingRequired: onboardingRequired,
	}, nil
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
		return nil, domainerrors.ErrValidationFailed.WithDetails("store_name is required for merchant sign-in")
	}

	profile := &entity.MerchantProfile{
		StoreName:          storeName,
		VerificationStatus: entity.MerchantVerificationStatusUnverified,
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
		return err
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

// storeRefreshToken stores the refresh token in the database.
func (srv *userService) storeRefreshToken(
	ctx context.Context,
	repoFactory repository.RepositoryFactory,
	userID uuid.UUID,
	refreshTokenString string,
	familyID uuid.UUID,
) error {
	refreshRepo := repoFactory.RefreshTokenRepo()
	userRepo := repoFactory.UserRepo()

	// Defensive: check maxActiveSessions here because storeRefreshToken is called
	// from multiple sites (e.g. handleOAuthUserAuth), not only persistLoginRefreshToken.
	if srv.maxActiveSessions > 0 {
		if err := userRepo.AcquireSessionMutex(ctx, userID); err != nil {
			return err
		}

		activeSessions, err := refreshRepo.CountActiveSessionsByUserID(ctx, userID)
		if err != nil {
			return err
		}
		if activeSessions >= srv.maxActiveSessions {
			return fmt.Errorf("active session limit exceeded: %w", domainerrors.ErrSessionLimitExceeded)
		}
	}

	return srv.storeRefreshTokenWithRepo(ctx, refreshRepo, userID, refreshTokenString, familyID)
}

func (srv *userService) storeRefreshTokenDirect(
	ctx context.Context,
	userID uuid.UUID,
	refreshTokenString string,
	familyID uuid.UUID,
) error {
	return srv.storeRefreshTokenWithRepo(ctx, srv.refreshTokenRepo, userID, refreshTokenString, familyID)
}

func (srv *userService) storeRefreshTokenWithRepo(
	ctx context.Context,
	refreshRepo repository.RefreshTokenRepository,
	userID uuid.UUID,
	refreshTokenString string,
	familyID uuid.UUID,
) error {
	// Hash the refresh token
	refreshTokenHash := srv.tokenService.HashToken(refreshTokenString)

	newRefreshToken := &entity.RefreshToken{
		UserID:    userID,
		TokenHash: refreshTokenHash,
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(srv.tokenService.GetRefreshTokenDuration()),
	}

	if err := refreshRepo.CreateRefreshToken(ctx, newRefreshToken); err != nil {
		return err
	}

	return nil
}

// LogoutAllDevices handles the process of invalidating all user sessions by revoking all refresh token families.
func (srv *userService) LogoutAllDevices(ctx context.Context, userID uuid.UUID) error {
	srv.log(ctx).Info("Attempting to log out from all devices", slog.String("user_id", userID.String()))

	// Single operation - use direct repository instance
	if err := srv.refreshTokenRepo.RevokeTokenFamiliesByUserID(ctx, userID); err != nil {
		srv.log(ctx).Error("Failed to revoke all refresh token families", slog.String("error", err.Error()), slog.String("user_id", userID.String()))

		return err
	}
	srv.log(ctx).Info("Successfully logged out from all devices", slog.String("user_id", userID.String()))

	return nil
}

// GetActiveSessions retrieves all active sessions for a user.
func (srv *userService) GetActiveSessions(ctx context.Context, userID uuid.UUID) ([]*entity.RefreshToken, error) {
	srv.log(ctx).Debug("Getting active sessions", slog.String("user_id", userID.String()))

	// Single query operation - use direct repository instance
	sessions, err := srv.refreshTokenRepo.FindRefreshTokensByUserID(ctx, userID)
	if err != nil {
		srv.log(ctx).Error("Failed to get active sessions", slog.String("error", err.Error()), slog.String("user_id", userID.String()))

		return nil, err
	}

	return sessions, nil
}

// RevokeSession revokes a specific token family by refresh token ID.
func (srv *userService) RevokeSession(ctx context.Context, userID, tokenID uuid.UUID) error {
	srv.log(ctx).Info("Attempting to revoke session", slog.String("user_id", userID.String()), slog.String("token_id", tokenID.String()))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		refreshRepo := repoFactory.RefreshTokenRepo()
		userRepo := repoFactory.UserRepo()

		if err := userRepo.AcquireSessionMutex(ctx, userID); err != nil {
			return err
		}

		// Verify the token belongs to the user before deleting
		token, err := refreshRepo.FindRefreshTokenByID(ctx, tokenID)
		if err != nil {
			return err
		}

		if token.UserID != userID {
			return fmt.Errorf("token does not belong to user: %w", domainerrors.ErrForbidden)
		}

		if err := refreshRepo.RevokeTokenFamily(ctx, token.FamilyID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to revoke session", slog.String("error", err.Error()), slog.String("user_id", userID.String()), slog.String("token_id", tokenID.String()))

		return err
	}
	srv.log(ctx).Info("Successfully revoked session", slog.String("user_id", userID.String()), slog.String("token_id", tokenID.String()))

	return nil
}

// LinkGoogleAccount links a Google account to an existing user account.
func (srv *userService) LinkGoogleAccount(ctx context.Context, userID uuid.UUID, idToken string) error {
	srv.log(ctx).Info("Linking Google account to existing user", slog.String("user_id", userID.String()))

	// 1. Verify the Google ID token
	oauthUser, err := srv.googleAuthService.VerifyIDToken(ctx, idToken)
	if err != nil {
		return fmt.Errorf("failed to verify Google ID token: %w", err)
	}

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		return srv.performGoogleAccountLinking(ctx, repoFactory, userID, oauthUser)
	})

	if err != nil {
		srv.log(ctx).Error("Failed to link Google account", slog.String("error", err.Error()), slog.String("user_id", userID.String()))

		return fmt.Errorf("failed to link Google account: %w", err)
	}
	srv.log(ctx).Info("Successfully linked Google account", slog.String("user_id", userID.String()))

	return nil
}

// performGoogleAccountLinking handles the core logic for linking a Google account
func (srv *userService) performGoogleAccountLinking(ctx context.Context, repoFactory repository.RepositoryFactory, userID uuid.UUID, oauthUser *service.OAuthUser) error {
	authRepo := repoFactory.AuthRepo()
	userRepo := repoFactory.UserRepo()

	// 1. Verify the user exists
	user, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
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
			slog.String("user_id", userID.String()),
			slog.String("user_email", user.Email),
			slog.String("google_email", oauthUser.Email))
	}

	return nil
}

// checkGoogleAccountConflicts checks if the Google account is already linked to another user
func (srv *userService) checkGoogleAccountConflicts(ctx context.Context, authRepo repository.AuthRepository, userID uuid.UUID, googleUserID string) error {
	existingAuth, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeGoogle, googleUserID)
	if err != nil && !errors.Is(err, domainerrors.ErrAuthNotFound) {
		return err
	}

	if existingAuth != nil {
		if existingAuth.UserID == userID {
			return domainerrors.ErrConflict.WithDetails("google account already linked to this user")
		}

		return domainerrors.ErrConflict.WithDetails("google account already linked to another user")
	}

	return nil
}

// createOrUpdateGoogleAuth creates or updates the Google authentication for the user
func (srv *userService) createOrUpdateGoogleAuth(ctx context.Context, authRepo repository.AuthRepository, userID uuid.UUID, googleUserID string) error {
	userGoogleAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle)
	if err != nil && !errors.Is(err, domainerrors.ErrAuthNotFound) {
		return err
	}

	if userGoogleAuth != nil {
		// Update existing Google authentication
		userGoogleAuth.ProviderUserID = googleUserID
		if err := authRepo.UpdateAuthentication(ctx, userGoogleAuth); err != nil {
			return err
		}
	} else {
		// Create new Google authentication
		newAuth := &entity.Authentication{
			UserID:         userID,
			Provider:       entity.ProviderTypeGoogle,
			ProviderUserID: googleUserID,
		}

		if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
			return err
		}
	}

	return nil
}

// UnlinkGoogleAccount removes the Google authentication method from a user account.
func (srv *userService) UnlinkGoogleAccount(ctx context.Context, userID uuid.UUID) error {
	srv.log(ctx).Info("Unlinking Google account from user", slog.String("user_id", userID.String()))

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		authRepo := repoFactory.AuthRepo()

		// 1. Find the user's Google authentication
		googleAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, entity.ProviderTypeGoogle)
		if err != nil {
			if errors.Is(err, domainerrors.ErrAuthNotFound) {
				return domainerrors.ErrNotFound.WithDetails("google account not linked to this user")
			}

			return err
		}

		// 2. Check if user has other authentication methods
		allAuths, err := authRepo.ListAuthenticationsByUserID(ctx, userID)
		if err != nil {
			return err
		}

		if len(allAuths) <= 1 {
			return domainerrors.ErrValidationFailed.WithDetails("cannot unlink last authentication method")
		}

		// 3. Delete the Google authentication
		if err := authRepo.DeleteAuthentication(ctx, googleAuth.ID); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		srv.log(ctx).Error("Failed to unlink Google account", slog.String("error", err.Error()), slog.String("user_id", userID.String()))

		return err
	}
	srv.log(ctx).Info("Successfully unlinked Google account", slog.String("user_id", userID.String()))

	return nil
}
