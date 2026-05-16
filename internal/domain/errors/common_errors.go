package errors

import "net/http"

var (
	ErrPersistenceFailed      = NewBaseError(http.StatusInternalServerError, "PERSISTENCE_FAILED", "資料存取失敗", "")
	ErrInternalError          = NewBaseError(http.StatusInternalServerError, "INTERNAL_ERROR", "系統內部錯誤", "")
	ErrTransactionFailed      = NewBaseError(http.StatusInternalServerError, "TRANSACTION_FAILED", "資料庫交易失敗", "")
	ErrInvalidInput           = NewBaseError(http.StatusBadRequest, "INVALID_INPUT", "輸入格式錯誤", "")
	ErrInvalidID              = NewBaseError(http.StatusBadRequest, "INVALID_ID", "識別碼格式錯誤", "")
	ErrValidationFailed       = NewBaseError(http.StatusBadRequest, "VALIDATION_FAILED", "輸入資料驗證失敗", "")
	ErrForbidden              = NewBaseError(http.StatusForbidden, "FORBIDDEN", "存取被拒絕", "")
	ErrForbiddenResourceOwner = NewBaseError(http.StatusForbidden, "FORBIDDEN_RESOURCE_OWNER", "您沒有權限存取此資源", "")
	ErrUnauthorized           = NewBaseError(http.StatusUnauthorized, "UNAUTHORIZED", "未授權的操作", "")
	ErrNotFound               = NewBaseError(http.StatusNotFound, "NOT_FOUND", "找不到資源", "")
	ErrConflict               = NewBaseError(http.StatusConflict, "CONFLICT", "資源衝突", "")
	ErrForbiddenHost          = NewBaseError(http.StatusForbidden, "FORBIDDEN_HOST", "不允許的網域", "")
	ErrForbiddenOrigin        = NewBaseError(http.StatusForbidden, "FORBIDDEN_ORIGIN", "不允許的來源", "")
)
