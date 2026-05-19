package impl

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strings"
	"time"

	"radar/internal/domain/entity"
	domainerrors "radar/internal/domain/errors"
	"radar/internal/domain/policy"
	"radar/internal/domain/repository"
	"radar/internal/usecase"

	"github.com/google/uuid"
)

const (
	lockoutNotificationTitle = "帳號安全通知"
	lockoutNotificationBody  = "您的帳號因多次登入失敗已被暫時鎖定。如果這不是您的操作，請立即更改密碼。"
	securityAlertTitle       = "帳號安全通知"
	securitySessionAlertBody = "偵測到可疑的登入活動，您的所有工作階段已被撤銷。請重新登入並確認帳號安全。"
)

func (srv *userService) checkLoginThrottle(ctx context.Context, email string) (*entity.LoginAttempt, error) {
	attemptKey := entity.NormalizeEmail(email)

	if err := srv.loginAttemptRepo.DecayLockoutCounts(ctx, srv.loginThrottleCfg.LockoutDecayDays); err != nil {
		return nil, err
	}

	userID, err := srv.resolveLoginAttemptUserID(ctx, srv.authRepo, attemptKey)
	if err != nil {
		return nil, err
	}

	attempt, err := srv.loginAttemptRepo.FindOrCreateByAttemptKey(ctx, attemptKey, userID)
	if err != nil {
		return nil, err
	}
	if attempt.UserID == nil && userID != nil {
		attempt.UserID = userID
	}

	if attempt.LockedUntil == nil || !attempt.LockedUntil.After(time.Now()) {
		return attempt, nil
	}

	return attempt, &usecase.LockoutError{
		RetryAfterSeconds: retryAfterSeconds(*attempt.LockedUntil),
		Err:               domainerrors.ErrInvalidCredentials,
	}
}

func (srv *userService) recordLoginFailure(ctx context.Context, email string, userID *uuid.UUID) error {
	attemptKey := entity.NormalizeEmail(email)
	var attempt *entity.LoginAttempt
	lockedBefore := false

	err := srv.txManager.Execute(ctx, func(repoFactory repository.RepositoryFactory) error {
		loginAttemptRepo := repoFactory.LoginAttemptRepo()

		var err error
		attempt, err = loginAttemptRepo.FindOrCreateByAttemptKeyForUpdate(ctx, attemptKey, userID)
		if err != nil {
			return err
		}

		now := time.Now()
		if isLoginAttemptLocked(attempt, now) {
			lockedBefore = true

			return nil
		}

		srv.applyLoginFailure(attempt, userID, now)

		return loginAttemptRepo.Save(ctx, attempt)
	})
	if err != nil {
		return err
	}

	if attempt.LockedUntil == nil || !attempt.LockedUntil.After(time.Now()) || attempt.FailedCount != 0 {
		return nil
	}

	if userID != nil && !lockedBefore {
		srv.sendLockoutNotification(ctx, *userID, *attempt.LockedUntil)
	}

	return &usecase.LockoutError{
		RetryAfterSeconds: retryAfterSeconds(*attempt.LockedUntil),
		Err:               domainerrors.ErrInvalidCredentials,
	}
}

func (srv *userService) recordLoginSuccess(ctx context.Context, email string) error {
	return srv.loginAttemptRepo.ResetOnSuccess(ctx, entity.NormalizeEmail(email))
}

func firstNonNilUUID(current, next *uuid.UUID) *uuid.UUID {
	if current != nil {
		return current
	}

	return next
}

func isLoginAttemptLocked(attempt *entity.LoginAttempt, now time.Time) bool {
	return attempt.LockedUntil != nil && attempt.LockedUntil.After(now)
}

func (srv *userService) applyLoginFailure(attempt *entity.LoginAttempt, userID *uuid.UUID, now time.Time) {
	attempt.UserID = firstNonNilUUID(attempt.UserID, userID)
	attempt.LastFailedAt = &now
	attempt.UpdatedAt = now

	if attempt.FailedCount+1 < srv.loginThrottleCfg.MaxAttempts {
		attempt.FailedCount++

		return
	}

	lockoutMinutes := srv.loginThrottlePolicy.LockoutMinutes(attempt.LockoutCount)
	lockedUntil := now.Add(time.Duration(lockoutMinutes) * time.Minute)

	attempt.LockoutCount++
	attempt.LockedUntil = &lockedUntil
	attempt.LastLockoutAt = &now
	attempt.FailedCount = 0
}

func (srv *userService) sendLockoutNotification(ctx context.Context, userID uuid.UUID, lockedUntil time.Time) {
	logger := srv.log(ctx)

	go func() {
		notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), srv.notificationTimeout)
		defer cancel()

		devices, err := srv.deviceRepo.FindDevicesByUser(notifyCtx, userID, repository.DeviceListFilter{
			OnlyHealthy:       true,
			HealthyWindowDays: policy.DefaultDevicePolicy().HealthyWindowDays,
		})
		if err != nil {
			logger.Warn("Failed to load devices for login lockout notification", slog.String("user_id", userID.String()), slog.String("error", err.Error()))

			return
		}

		tokens := make([]string, 0, len(devices))
		for _, device := range devices {
			if token := strings.TrimSpace(device.FCMToken); token != "" {
				tokens = append(tokens, token)
			}
		}
		if len(tokens) == 0 {
			return
		}

		data := map[string]string{
			"event":        "login_lockout",
			"locked_until": lockedUntil.UTC().Format(time.RFC3339),
		}

		if _, _, _, err := srv.notificationSvc.SendBatchNotification(
			notifyCtx,
			tokens,
			lockoutNotificationTitle,
			lockoutNotificationBody,
			data,
		); err != nil {
			logger.Warn("Failed to send login lockout notification", slog.String("user_id", userID.String()), slog.String("error", err.Error()))
		}
	}()
}

func (srv *userService) sendTokenReuseNotification(ctx context.Context, userID uuid.UUID) {
	logger := srv.log(ctx)

	go func() {
		notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), srv.notificationTimeout)
		defer cancel()

		devices, err := srv.deviceRepo.FindDevicesByUser(notifyCtx, userID, repository.DeviceListFilter{
			OnlyHealthy:       true,
			HealthyWindowDays: policy.DefaultDevicePolicy().HealthyWindowDays,
		})
		if err != nil {
			logger.Warn("Failed to load devices for refresh token reuse notification", slog.String("user_id", userID.String()), slog.String("error", err.Error()))

			return
		}

		tokens := make([]string, 0, len(devices))
		for _, device := range devices {
			if token := strings.TrimSpace(device.FCMToken); token != "" {
				tokens = append(tokens, token)
			}
		}
		if len(tokens) == 0 {
			return
		}

		data := map[string]string{
			"event": "refresh_token_reuse_detected",
		}

		if _, _, _, err := srv.notificationSvc.SendBatchNotification(
			notifyCtx,
			tokens,
			securityAlertTitle,
			securitySessionAlertBody,
			data,
		); err != nil {
			logger.Warn("Failed to send refresh token reuse notification", slog.String("user_id", userID.String()), slog.String("error", err.Error()))
		}
	}()
}

func (srv *userService) resolveLoginAttemptUserID(ctx context.Context, authRepo repository.AuthRepository, attemptKey string) (*uuid.UUID, error) {
	authRecord, err := authRepo.FindAuthentication(ctx, entity.ProviderTypeEmail, attemptKey)
	if err == nil {
		return &authRecord.UserID, nil
	}
	if errors.Is(err, domainerrors.ErrAuthNotFound) {
		return nil, nil
	}

	return nil, err
}

func retryAfterSeconds(lockedUntil time.Time) int {
	seconds := int(math.Ceil(time.Until(lockedUntil).Seconds()))
	if seconds < 1 {
		return 1
	}

	return seconds
}
