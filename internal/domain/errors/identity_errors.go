package errors

import "net/http"

var (
	ErrUserNotFound              = NewBaseError(http.StatusNotFound, "USER_NOT_FOUND", "找不到該使用者", "")
	ErrUserAlreadyExists         = NewBaseError(http.StatusConflict, "USER_ALREADY_EXISTS", "此電子郵件已被註冊", "")
	ErrUserCreateFailed          = NewBaseError(http.StatusInternalServerError, "USER_CREATE_FAILED", "建立使用者失敗", "")
	ErrUserUpdateFailed          = NewBaseError(http.StatusInternalServerError, "USER_UPDATE_FAILED", "更新使用者失敗", "")
	ErrAuthNotFound              = NewBaseError(http.StatusNotFound, "AUTH_NOT_FOUND", "找不到認證方式", "")
	ErrAuthAlreadyExists         = NewBaseError(http.StatusConflict, "AUTH_ALREADY_EXISTS", "認證方式已存在", "")
	ErrAuthCreateFailed          = NewBaseError(http.StatusInternalServerError, "AUTH_CREATE_FAILED", "建立認證方式失敗", "")
	ErrAuthUpdateFailed          = NewBaseError(http.StatusInternalServerError, "AUTH_UPDATE_FAILED", "更新認證方式失敗", "")
	ErrInvalidCredentials        = NewBaseError(http.StatusUnauthorized, "INVALID_CREDENTIALS", "電子郵件或密碼錯誤", "")
	ErrRefreshTokenInvalid       = NewBaseError(http.StatusUnauthorized, "REFRESH_TOKEN_INVALID", "無效或已過期的重新整理權杖", "")
	ErrPasswordHashFailed        = NewBaseError(http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "密碼處理錯誤", "")
	ErrPasswordStrength          = NewBaseError(http.StatusBadRequest, "PASSWORD_STRENGTH", "密碼強度不足", "")
	ErrPasswordForbiddenWords    = NewBaseError(http.StatusBadRequest, "PASSWORD_FORBIDDEN_WORDS", "密碼包含禁止使用的字詞或模式", "")
	ErrOAuthFailed               = NewBaseError(http.StatusUnauthorized, "OAUTH_FAILED", "OAuth 認證失敗", "")
	ErrOAuthCodeInvalid          = NewBaseError(http.StatusBadRequest, "OAUTH_CODE_INVALID", "無效的授權碼", "")
	ErrOAuthTokenInvalid         = NewBaseError(http.StatusBadRequest, "OAUTH_TOKEN_INVALID", "無效的 ID 權杖", "")
	ErrRefreshTokenNotFound      = NewBaseError(http.StatusNotFound, "REFRESH_TOKEN_NOT_FOUND", "找不到重新整理權杖", "")
	ErrRefreshTokenExpired       = NewBaseError(http.StatusUnauthorized, "REFRESH_TOKEN_EXPIRED", "重新整理權杖已過期", "")
	ErrRefreshTokenAlreadyExists = NewBaseError(
		http.StatusConflict,
		"REFRESH_TOKEN_ALREADY_EXISTS",
		"重新整理權杖已存在",
		"",
	)
	ErrRefreshTokenCreateFailed = NewBaseError(
		http.StatusInternalServerError,
		"REFRESH_TOKEN_CREATE_FAILED",
		"建立重新整理權杖失敗",
		"",
	)
	ErrRefreshTokenUpdateFailed = NewBaseError(
		http.StatusInternalServerError,
		"REFRESH_TOKEN_UPDATE_FAILED",
		"更新重新整理權杖失敗",
		"",
	)
	ErrSessionLimitExceeded = NewBaseError(http.StatusTooManyRequests, "SESSION_LIMIT_EXCEEDED", "已達到最大同時登入裝置數量", "")
)
