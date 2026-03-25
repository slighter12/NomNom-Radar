package impl

import (
	"context"
	"log/slog"
	"strings"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	domainservice "radar/internal/domain/service"
	"radar/internal/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
)

type authMethod string

const (
	authMethodEmailPassword authMethod = "email_password"
	authMethodOAuth         authMethod = "oauth"
)

type authIntent string

const (
	authIntentLogin    authIntent = "login"
	authIntentRegister authIntent = "register"
)

func merchantOnboardingRequiredFields() []string {
	return []string{"store_name", "business_license"}
}

type authRequest struct {
	Method        authMethod
	Intent        authIntent
	Provider      entity.ProviderType
	RequestedRole entity.Role
	Name          string
	Email         string
	Password      string
	IDToken       string
	MerchantSeed  *merchantProfileSeed
}

type verifiedIdentity struct {
	Provider       entity.ProviderType
	ProviderUserID string
	Email          string
	Name           string
	EmailVerified  bool
	PasswordHash   string
}

type authResolution struct {
	User               *entity.User
	OnboardingRequired bool
	// StoredPasswordHash is set when an existing email auth record is found,
	// so that bcrypt check can happen outside the transaction.
	StoredPasswordHash string
}

func (srv *userService) authenticate(ctx context.Context, req *authRequest) (*usecase.AuthResult, error) {
	verifiedIdentity, err := srv.verifyIdentity(ctx, req)
	if err != nil {
		return nil, err
	}

	var resolution *authResolution

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		var resolveErr error
		resolution, resolveErr = srv.resolveAuthRequest(ctx, repoFactory, req, verifiedIdentity)

		return resolveErr
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute authentication transaction")
	}

	// Bcrypt check is CPU-bound; run it outside the transaction to avoid holding
	// the DB connection while hashing.
	if resolution.StoredPasswordHash != "" {
		if !srv.hasher.Check(req.Password, resolution.StoredPasswordHash) {
			return nil, errors.Wrap(domainerrors.ErrInvalidCredentials, "invalid credentials")
		}
	}

	if resolution.OnboardingRequired {
		return srv.buildOnboardingRequiredResult(resolution.User, req.RequestedRole)
	}

	return srv.buildAuthenticatedResult(ctx, resolution.User)
}

func (srv *userService) verifyIdentity(ctx context.Context, req *authRequest) (*verifiedIdentity, error) {
	switch req.Method {
	case authMethodEmailPassword:
		return srv.verifyEmailIdentity(ctx, req)
	case authMethodOAuth:
		return srv.verifyOAuthIdentity(ctx, req)
	default:
		return nil, errors.Wrap(domainerrors.ErrValidationFailed, "unsupported authentication method")
	}
}

func (srv *userService) verifyEmailIdentity(ctx context.Context, req *authRequest) (*verifiedIdentity, error) {
	identity := &verifiedIdentity{
		Provider:       entity.ProviderTypeEmail,
		ProviderUserID: req.Email,
		Email:          req.Email,
		Name:           req.Name,
		EmailVerified:  true,
	}

	if req.Intent != authIntentRegister {
		return identity, nil
	}

	if err := srv.hasher.ValidatePasswordStrength(req.Password); err != nil {
		srv.log(ctx).Warn(
			"Password validation failed during registration",
			slog.String("email_masked", maskEmailForLog(req.Email)),
			slog.Any("error", err),
		)

		return nil, errors.Wrap(domainerrors.ErrValidationFailed, "password does not meet security requirements")
	}

	hashedPassword, err := srv.hasher.Hash(req.Password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to hash password during registration")
	}
	identity.PasswordHash = hashedPassword

	return identity, nil
}

func (srv *userService) verifyOAuthIdentity(ctx context.Context, req *authRequest) (*verifiedIdentity, error) {
	oauthUser, err := srv.googleAuthService.VerifyIDToken(ctx, req.IDToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to verify OAuth ID token")
	}

	provider := oauthUser.Provider
	if provider == "" {
		provider = srv.googleAuthService.GetProvider()
	}

	if !oauthUser.EmailVerified {
		return nil, errors.Wrap(domainerrors.ErrValidationFailed, "oauth provider email must be verified")
	}

	return &verifiedIdentity{
		Provider:       provider,
		ProviderUserID: oauthUser.ID,
		Email:          entity.NormalizeEmail(oauthUser.Email),
		Name:           oauthUser.Name,
		EmailVerified:  oauthUser.EmailVerified,
	}, nil
}

func (srv *userService) resolveAuthRequest(
	ctx context.Context,
	repoFactory repository.RepositoryFactory,
	req *authRequest,
	identity *verifiedIdentity,
) (*authResolution, error) {
	authRepo := repoFactory.AuthRepo()
	userRepo := repoFactory.UserRepo()

	authRecord, err := authRepo.FindAuthentication(ctx, identity.Provider, identity.ProviderUserID)
	if err != nil && !errors.Is(err, repository.ErrAuthNotFound) {
		return nil, errors.Wrap(err, "failed to find authentication")
	}

	if err == nil {
		return srv.resolveExistingLinkedUser(ctx, userRepo, req, identity, authRecord)
	}

	if req.Method == authMethodEmailPassword && req.Intent == authIntentLogin {
		return nil, errors.Wrap(domainerrors.ErrInvalidCredentials, "login failed")
	}

	return srv.resolveUnlinkedIdentity(ctx, userRepo, authRepo, req, identity)
}

func (srv *userService) resolveExistingLinkedUser(
	ctx context.Context,
	userRepo repository.UserRepository,
	req *authRequest,
	identity *verifiedIdentity,
	authRecord *entity.Authentication,
) (*authResolution, error) {
	user, err := userRepo.FindByID(ctx, authRecord.UserID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user by id")
	}

	onboardingRequired, err := srv.ensureRequestedRole(ctx, userRepo, user, req, identity)
	if err != nil {
		return nil, err
	}

	resolution := &authResolution{
		User:               user,
		OnboardingRequired: onboardingRequired,
	}

	// Defer password verification to outside the transaction (bcrypt is CPU-bound).
	if req.Method == authMethodEmailPassword {
		resolution.StoredPasswordHash = authRecord.PasswordHash
	}

	return resolution, nil
}

func (srv *userService) resolveUnlinkedIdentity(
	ctx context.Context,
	userRepo repository.UserRepository,
	authRepo repository.AuthRepository,
	req *authRequest,
	identity *verifiedIdentity,
) (*authResolution, error) {
	existingUser, err := userRepo.FindByEmail(ctx, identity.Email)
	switch {
	case err == nil:
		return srv.resolveExistingEmailAccount(ctx, userRepo, authRepo, req, identity, existingUser)
	case errors.Is(err, repository.ErrUserNotFound):
		return srv.createNewUserForIdentity(ctx, userRepo, authRepo, req, identity)
	default:
		return nil, errors.Wrap(err, "failed to find user by email")
	}
}

func (srv *userService) resolveExistingEmailAccount(
	ctx context.Context,
	userRepo repository.UserRepository,
	authRepo repository.AuthRepository,
	req *authRequest,
	identity *verifiedIdentity,
	existingUser *entity.User,
) (*authResolution, error) {
	if err := linkIdentityToExistingUser(ctx, authRepo, req, identity, existingUser.ID); err != nil {
		return nil, err
	}

	onboardingRequired, err := srv.ensureRequestedRole(ctx, userRepo, existingUser, req, identity)
	if err != nil {
		return nil, err
	}

	return &authResolution{
		User:               existingUser,
		OnboardingRequired: onboardingRequired,
	}, nil
}

func (srv *userService) createNewUserForIdentity(
	ctx context.Context,
	userRepo repository.UserRepository,
	authRepo repository.AuthRepository,
	req *authRequest,
	identity *verifiedIdentity,
) (*authResolution, error) {
	user, onboardingRequired, err := srv.createUserSkeleton(ctx, userRepo, req, identity)
	if err != nil {
		return nil, err
	}

	if req.Method == authMethodOAuth {
		if err := createOAuthAuthentication(ctx, authRepo, user.ID, identity.Provider, identity.ProviderUserID); err != nil {
			return nil, err
		}
	} else {
		if err := createEmailAuthentication(ctx, authRepo, user.ID, identity.Email, identity.PasswordHash); err != nil {
			return nil, err
		}
	}

	return &authResolution{
		User:               user,
		OnboardingRequired: onboardingRequired,
	}, nil
}

func (srv *userService) createUserSkeleton(
	ctx context.Context,
	userRepo repository.UserRepository,
	req *authRequest,
	identity *verifiedIdentity,
) (*entity.User, bool, error) {
	switch req.RequestedRole {
	case entity.RoleMerchant:
		if req.MerchantSeed == nil || !req.MerchantSeed.isComplete() {
			user := &entity.User{
				Name:  identity.Name,
				Email: identity.Email,
			}
			if err := userRepo.Create(ctx, user); err != nil {
				return nil, false, errors.Wrap(err, "failed to create onboarding user")
			}

			return user, true, nil
		}

		cfg := buildMerchantRegistrationConfig(identity.Name, identity.Email, "", *req.MerchantSeed)
		user, err := srv.createUserWithProfile(ctx, cfg, userRepo)
		if err != nil {
			return nil, false, err
		}

		return user, false, nil
	default:
		cfg := buildUserRegistrationConfig(identity.Name, identity.Email, "")
		user, err := srv.createUserWithProfile(ctx, cfg, userRepo)
		if err != nil {
			return nil, false, err
		}

		return user, false, nil
	}
}

func (srv *userService) ensureRequestedRole(
	ctx context.Context,
	userRepo repository.UserRepository,
	user *entity.User,
	req *authRequest,
	identity *verifiedIdentity,
) (bool, error) {
	switch req.RequestedRole {
	case entity.RoleMerchant:
		if userHasMerchantProfile(user) {
			return false, nil
		}
		if req.MerchantSeed == nil || !req.MerchantSeed.isComplete() {
			return true, nil
		}

		cfg := buildMerchantRegistrationConfig(identity.Name, identity.Email, "", *req.MerchantSeed)
		if err := srv.syncExistingAccountProfile(ctx, cfg, userRepo, user, false); err != nil {
			return false, err
		}

		return false, nil
	case entity.RoleUser:
		if userHasUserProfile(user) {
			return false, nil
		}

		cfg := buildUserRegistrationConfig(identity.Name, identity.Email, "")
		if err := srv.syncExistingAccountProfile(ctx, cfg, userRepo, user, false); err != nil {
			return false, err
		}

		return false, nil
	default:
		return false, nil
	}
}

func (srv *userService) buildAuthenticatedResult(ctx context.Context, user *entity.User) (*usecase.AuthResult, error) {
	roles := srv.extractUserRoles(user)

	accessToken, refreshToken, err := srv.tokenService.GenerateTokens(user.ID, roles.ToStrings())
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate tokens")
	}

	if err := srv.persistLoginRefreshToken(ctx, user.ID, refreshToken); err != nil {
		return nil, errors.Wrap(err, "failed to create refresh token during authentication")
	}

	return &usecase.AuthResult{
		Status:       usecase.AuthStatusAuthenticated,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func (srv *userService) buildOnboardingRequiredResult(user *entity.User, requestedRole entity.Role) (*usecase.AuthResult, error) {
	onboardingToken, err := srv.tokenService.GenerateOnboardingToken(user.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate onboarding token")
	}

	return &usecase.AuthResult{
		Status:          usecase.AuthStatusOnboardingRequired,
		OnboardingToken: onboardingToken,
		RequestedRole:   requestedRole.String(),
		RequiredFields:  merchantOnboardingRequiredFields(),
	}, nil
}

func (srv *userService) CompleteMerchantOnboarding(ctx context.Context, input *usecase.CompleteMerchantOnboardingInput) (*usecase.AuthResult, error) {
	claims, err := srv.tokenService.ValidateToken(input.OnboardingToken)
	if err != nil {
		return nil, errors.Wrap(domainerrors.ErrUnauthorized, "invalid onboarding token")
	}
	if claims.Type != domainservice.TokenTypeOnboarding {
		return nil, errors.Wrap(domainerrors.ErrUnauthorized, "invalid onboarding token type")
	}

	seed := merchantProfileSeed{
		StoreName:       strings.TrimSpace(input.StoreName),
		BusinessLicense: strings.TrimSpace(input.BusinessLicense),
	}

	var user *entity.User

	err = srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		userRepo := repoFactory.UserRepo()

		loadedUser, err := userRepo.FindByID(ctx, claims.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				return errors.Wrap(domainerrors.ErrNotFound, "user not found")
			}

			return errors.Wrap(err, "failed to find user")
		}

		user = loadedUser
		if userHasMerchantProfile(user) {
			return errors.Wrap(domainerrors.ErrConflict, "merchant onboarding already completed")
		}

		cfg := buildMerchantRegistrationConfig(user.Name, user.Email, "", seed)
		if err := srv.syncExistingAccountProfile(ctx, cfg, userRepo, user, false); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to complete merchant onboarding")
	}

	return srv.buildAuthenticatedResult(ctx, user)
}

func buildUserRegistrationConfig(name, email, password string) *registrationConfig {
	return &registrationConfig{
		Name:     name,
		Email:    email,
		Password: password,
		Role:     entity.RoleUser,
		BuildNewUser: func() (*entity.User, error) {
			return buildNewUserEntity(name, email), nil
		},
		AttachProfile: func(user *entity.User) error {
			attachUserProfile(user)

			return nil
		},
		HasProfile: userHasUserProfile,
		ProfileExistsError: func() error {
			return domainerrors.ErrUserAlreadyExists.WrapMessage("user profile already registered for this account")
		},
	}
}

func buildMerchantRegistrationConfig(name, email, password string, seed merchantProfileSeed) *registrationConfig {
	return &registrationConfig{
		Name:     name,
		Email:    email,
		Password: password,
		Role:     entity.RoleMerchant,
		BuildNewUser: func() (*entity.User, error) {
			return buildNewMerchantEntityFromSeed(name, email, seed)
		},
		AttachProfile: attachMerchantProfileFromSeed(seed),
		HasProfile:    userHasMerchantProfile,
		ProfileExistsError: func() error {
			return errors.Wrap(domainerrors.ErrMerchantAlreadyExists, "merchant profile already registered for this account")
		},
	}
}

func createEmailAuthentication(ctx context.Context, authRepo repository.AuthRepository, userID uuid.UUID, email, passwordHash string) error {
	newAuth := &entity.Authentication{
		UserID:         userID,
		Provider:       entity.ProviderTypeEmail,
		ProviderUserID: email,
		PasswordHash:   passwordHash,
	}

	if err := authRepo.CreateAuthentication(ctx, newAuth); err != nil {
		return errors.Wrap(err, "failed to create email authentication")
	}

	return nil
}

func linkIdentityToExistingUser(
	ctx context.Context,
	authRepo repository.AuthRepository,
	req *authRequest,
	identity *verifiedIdentity,
	userID uuid.UUID,
) error {
	switch req.Method {
	case authMethodOAuth:
		if !identity.EmailVerified {
			return errors.Wrap(domainerrors.ErrConflict, "account with this email already exists")
		}

		return ensureOAuthAuthLink(ctx, authRepo, userID, identity.Provider, identity.ProviderUserID)
	case authMethodEmailPassword:
		return errors.Wrap(domainerrors.ErrConflict, "account with this email already exists")
	default:
		return errors.Wrap(domainerrors.ErrConflict, "account with this email already exists")
	}
}

func ensureOAuthAuthLink(
	ctx context.Context,
	authRepo repository.AuthRepository,
	userID uuid.UUID,
	provider entity.ProviderType,
	providerUserID string,
) error {
	existingAuth, err := authRepo.FindAuthenticationByUserIDAndProvider(ctx, userID, provider)
	if err != nil && !errors.Is(err, repository.ErrAuthNotFound) {
		return errors.Wrap(err, "failed to find existing oauth authentication")
	}

	if existingAuth != nil {
		if existingAuth.ProviderUserID == providerUserID {
			return nil
		}

		return errors.Wrap(domainerrors.ErrConflict, "provider is already linked to a different account for this user")
	}

	return createOAuthAuthentication(ctx, authRepo, userID, provider, providerUserID)
}

func (s merchantProfileSeed) isComplete() bool {
	return strings.TrimSpace(s.StoreName) != "" && strings.TrimSpace(s.BusinessLicense) != ""
}
