package impl

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/repository"
	"radar/internal/domain/service"
	"radar/internal/errors"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sessionLimitTestTxManager struct {
	mu       sync.Mutex
	factory  repository.RepositoryFactory
	txStates map[context.Context]*sessionLimitTestTxState
}

type sessionLimitTestTxState struct {
	unlocks []func()
}

type sessionLimitContextKey string

const sessionLimitLoginContextKey sessionLimitContextKey = "login-index"

func (tm *sessionLimitTestTxManager) Execute(ctx context.Context, fn func(txRepoFactory repository.RepositoryFactory) error) error {
	state := &sessionLimitTestTxState{}

	tm.mu.Lock()
	tm.txStates[ctx] = state
	tm.mu.Unlock()

	defer func() {
		tm.mu.Lock()
		delete(tm.txStates, ctx)
		tm.mu.Unlock()

		// Mirror transactional behavior: release row locks only after commit/rollback.
		for i := len(state.unlocks) - 1; i >= 0; i-- {
			state.unlocks[i]()
		}
	}()

	return fn(tm.factory)
}

func (tm *sessionLimitTestTxManager) registerUnlock(ctx context.Context, unlockFn func()) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	state, ok := tm.txStates[ctx]
	if !ok {
		return errors.New("transaction state not found for context")
	}

	state.unlocks = append(state.unlocks, unlockFn)

	return nil
}

type sessionLimitTestRepoFactory struct {
	userRepo    repository.UserRepository
	authRepo    repository.AuthRepository
	refreshRepo repository.RefreshTokenRepository
}

func (f *sessionLimitTestRepoFactory) UserRepo() repository.UserRepository {
	return f.userRepo
}

func (f *sessionLimitTestRepoFactory) AuthRepo() repository.AuthRepository {
	return f.authRepo
}

func (f *sessionLimitTestRepoFactory) AddressRepo() repository.AddressRepository {
	return nil
}

func (f *sessionLimitTestRepoFactory) RefreshTokenRepo() repository.RefreshTokenRepository {
	return f.refreshRepo
}

type sessionLimitTestUserRepo struct {
	user      *entity.User
	lockCalls atomic.Int64
	txManager *sessionLimitTestTxManager
	locker    *sessionLimitRowLockManager
}

type sessionLimitRowLockManager struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
}

func newSessionLimitRowLockManager() *sessionLimitRowLockManager {
	return &sessionLimitRowLockManager{
		locks: make(map[uuid.UUID]*sync.Mutex),
	}
}

func (m *sessionLimitRowLockManager) lockUser(id uuid.UUID) func() {
	m.mu.Lock()
	lock, ok := m.locks[id]
	if !ok {
		lock = &sync.Mutex{}
		m.locks[id] = lock
	}
	m.mu.Unlock()

	lock.Lock()

	return lock.Unlock
}

func (r *sessionLimitTestUserRepo) FindByID(_ context.Context, id uuid.UUID) (*entity.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, repository.ErrUserNotFound
	}

	copied := *r.user

	return &copied, nil
}

func (r *sessionLimitTestUserRepo) AcquireSessionMutex(ctx context.Context, id uuid.UUID) error {
	if r.user == nil || r.user.ID != id {
		return repository.ErrUserNotFound
	}

	unlockFn := r.locker.lockUser(id)
	if err := r.txManager.registerUnlock(ctx, unlockFn); err != nil {
		unlockFn()

		return errors.Wrap(err, "failed to register transaction unlock")
	}

	r.lockCalls.Add(1)

	return nil
}

func (r *sessionLimitTestUserRepo) FindByEmail(_ context.Context, _ string) (*entity.User, error) {
	panic("not implemented")
}

func (r *sessionLimitTestUserRepo) Create(_ context.Context, _ *entity.User) error {
	panic("not implemented")
}

func (r *sessionLimitTestUserRepo) Update(_ context.Context, _ *entity.User) error {
	panic("not implemented")
}

type sessionLimitTestAuthRepo struct {
	authRecord *entity.Authentication
}

func (r *sessionLimitTestAuthRepo) CreateAuthentication(_ context.Context, _ *entity.Authentication) error {
	panic("not implemented")
}

func (r *sessionLimitTestAuthRepo) FindAuthentication(_ context.Context, provider entity.ProviderType, providerUserID string) (*entity.Authentication, error) {
	if r.authRecord == nil {
		return nil, repository.ErrAuthNotFound
	}

	if provider != r.authRecord.Provider || providerUserID != r.authRecord.ProviderUserID {
		return nil, repository.ErrAuthNotFound
	}

	copied := *r.authRecord

	return &copied, nil
}

func (r *sessionLimitTestAuthRepo) FindAuthenticationByUserIDAndProvider(_ context.Context, _ uuid.UUID, _ entity.ProviderType) (*entity.Authentication, error) {
	panic("not implemented")
}

func (r *sessionLimitTestAuthRepo) UpdateAuthentication(_ context.Context, _ *entity.Authentication) error {
	panic("not implemented")
}

func (r *sessionLimitTestAuthRepo) DeleteAuthentication(_ context.Context, _ uuid.UUID) error {
	panic("not implemented")
}

func (r *sessionLimitTestAuthRepo) ListAuthenticationsByUserID(_ context.Context, _ uuid.UUID) ([]*entity.Authentication, error) {
	panic("not implemented")
}

type sessionLimitTestRefreshRepo struct {
	mu     sync.Mutex
	active map[uuid.UUID]int
}

func newSessionLimitTestRefreshRepo() *sessionLimitTestRefreshRepo {
	return &sessionLimitTestRefreshRepo{
		active: make(map[uuid.UUID]int),
	}
}

func (r *sessionLimitTestRefreshRepo) CreateRefreshToken(_ context.Context, token *entity.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.active[token.UserID]++

	return nil
}

func (r *sessionLimitTestRefreshRepo) FindRefreshTokenByHash(_ context.Context, _ string) (*entity.RefreshToken, error) {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) FindRefreshTokenByID(_ context.Context, _ uuid.UUID) (*entity.RefreshToken, error) {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) FindRefreshTokensByUserID(_ context.Context, _ uuid.UUID) ([]*entity.RefreshToken, error) {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) UpdateRefreshToken(_ context.Context, _ *entity.RefreshToken) error {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) DeleteRefreshToken(_ context.Context, _ uuid.UUID) error {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) DeleteRefreshTokenByHash(_ context.Context, _ string) error {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) DeleteRefreshTokensByUserID(_ context.Context, _ uuid.UUID) error {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) DeleteExpiredRefreshTokens(_ context.Context) error {
	panic("not implemented")
}

func (r *sessionLimitTestRefreshRepo) CountActiveSessionsByUserID(_ context.Context, userID uuid.UUID) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.active[userID], nil
}

func (r *sessionLimitTestRefreshRepo) ActiveSessions(userID uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.active[userID]
}

type sessionLimitTestHasher struct{}

func (h *sessionLimitTestHasher) Hash(password string) (string, error) {
	return "hashed-" + password, nil
}

func (h *sessionLimitTestHasher) Check(password, hash string) bool {
	return password == "Password123!" && hash == "hashed-password"
}

func (h *sessionLimitTestHasher) ValidatePasswordStrength(_ string) error {
	return nil
}

type sessionLimitTestTokenService struct {
	seq atomic.Int64
}

func (s *sessionLimitTestTokenService) GenerateTokens(_ uuid.UUID, _ []string) (string, string, error) {
	n := s.seq.Add(1)

	return fmt.Sprintf("access-%d", n), fmt.Sprintf("refresh-%d", n), nil
}

func (s *sessionLimitTestTokenService) ValidateToken(_ string) (*service.Claims, error) {
	panic("not implemented")
}

func (s *sessionLimitTestTokenService) GetRefreshTokenDuration() time.Duration {
	return time.Hour
}

func (s *sessionLimitTestTokenService) HashToken(token string) string {
	return "hash-" + token
}

func (s *sessionLimitTestTokenService) RotateTokens(_ uuid.UUID, _ []string) (string, string, string, error) {
	panic("not implemented")
}

type sessionLimitTestOAuthService struct{}

func (s *sessionLimitTestOAuthService) VerifyIDToken(_ context.Context, _ string) (*service.OAuthUser, error) {
	panic("not implemented")
}

func (s *sessionLimitTestOAuthService) GetProvider() entity.ProviderType {
	return entity.ProviderTypeGoogle
}

func newSessionLimitTestService(t *testing.T, maxActiveSessions int) (usecase.UserUsecase, *sessionLimitTestUserRepo, *sessionLimitTestRefreshRepo) {
	t.Helper()

	txManager := &sessionLimitTestTxManager{
		txStates: make(map[context.Context]*sessionLimitTestTxState),
	}
	rowLockManager := newSessionLimitRowLockManager()

	userID := uuid.New()
	userRepo := &sessionLimitTestUserRepo{
		user: &entity.User{
			ID:          userID,
			Email:       "test@example.com",
			Name:        "session-limit-user",
			UserProfile: &entity.UserProfile{UserID: userID},
		},
		txManager: txManager,
		locker:    rowLockManager,
	}
	authRepo := &sessionLimitTestAuthRepo{
		authRecord: &entity.Authentication{
			UserID:         userID,
			Provider:       entity.ProviderTypeEmail,
			ProviderUserID: "test@example.com",
			PasswordHash:   "hashed-password",
		},
	}
	refreshRepo := newSessionLimitTestRefreshRepo()

	repoFactory := &sessionLimitTestRepoFactory{
		userRepo:    userRepo,
		authRepo:    authRepo,
		refreshRepo: refreshRepo,
	}
	txManager.factory = repoFactory

	logger := newDiscardLogger()
	service := NewUserService(UserServiceParams{
		TxManager:         txManager,
		UserRepo:          userRepo,
		AuthRepo:          authRepo,
		RefreshTokenRepo:  refreshRepo,
		Hasher:            &sessionLimitTestHasher{},
		TokenService:      &sessionLimitTestTokenService{},
		GoogleAuthService: &sessionLimitTestOAuthService{},
		Config:            newTestConfig(maxActiveSessions),
		Logger:            logger,
	})

	return service, userRepo, refreshRepo
}

func TestUserService_Login_EnforcesSessionLimit(t *testing.T) {
	uc, userRepo, refreshRepo := newSessionLimitTestService(t, 1)

	ctx := context.Background()
	input := &usecase.LoginInput{Email: "test@example.com", Password: "Password123!"}

	first, err := uc.Login(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, first)

	second, err := uc.Login(ctx, input)
	require.Error(t, err)
	assert.Nil(t, second)
	assert.True(t, errors.Is(err, domainerrors.ErrSessionLimitExceeded))
	assert.Equal(t, int64(2), userRepo.lockCalls.Load())
	assert.Equal(t, 1, refreshRepo.ActiveSessions(first.User.ID))
}

func TestUserService_Login_EnforcesSessionLimit_Concurrent(t *testing.T) {
	const (
		maxActiveSessions = 3
		concurrentLogins  = 12
	)

	uc, userRepo, refreshRepo := newSessionLimitTestService(t, maxActiveSessions)

	ctx := context.Background()
	input := &usecase.LoginInput{Email: "test@example.com", Password: "Password123!"}

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var limitExceededCount atomic.Int64
	var otherErrorCount atomic.Int64

	for i := 0; i < concurrentLogins; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			loginCtx := context.WithValue(ctx, sessionLimitLoginContextKey, i)
			out, err := uc.Login(loginCtx, input)
			switch {
			case err == nil:
				if out == nil {
					otherErrorCount.Add(1)

					return
				}
				successCount.Add(1)
			case errors.Is(err, domainerrors.ErrSessionLimitExceeded):
				limitExceededCount.Add(1)
			default:
				otherErrorCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int64(maxActiveSessions), successCount.Load())
	assert.Equal(t, int64(concurrentLogins-maxActiveSessions), limitExceededCount.Load())
	assert.Equal(t, int64(0), otherErrorCount.Load())
	assert.Equal(t, int64(concurrentLogins), userRepo.lockCalls.Load())
	assert.Equal(t, maxActiveSessions, refreshRepo.ActiveSessions(userRepo.user.ID))
}
