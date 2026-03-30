package errors

import "net/http"

var (
	ErrAddressNotFound           = NewBaseError(http.StatusNotFound, "ADDRESS_NOT_FOUND", "找不到該地址", "")
	ErrPrimaryAddressConflict    = NewBaseError(http.StatusConflict, "PRIMARY_ADDRESS_CONFLICT", "該使用者已設定主要地址", "")
	ErrAddressCreateFailed       = NewBaseError(http.StatusInternalServerError, "ADDRESS_CREATE_FAILED", "建立地址失敗", "")
	ErrAddressUpdateFailed       = NewBaseError(http.StatusInternalServerError, "ADDRESS_UPDATE_FAILED", "更新地址失敗", "")
	ErrAddressOwnershipViolation = NewBaseError(http.StatusForbidden, "ADDRESS_OWNERSHIP_VIOLATION", "您沒有權限存取此地址", "")
	ErrLocationLimitReached      = NewBaseError(http.StatusConflict, "LOCATION_LIMIT_REACHED", "已達位置數量上限", "")
	ErrDeviceNotFound            = NewBaseError(http.StatusNotFound, "DEVICE_NOT_FOUND", "找不到該裝置", "")
	ErrDeviceAlreadyExists       = NewBaseError(http.StatusConflict, "DEVICE_ALREADY_EXISTS", "裝置已存在", "")
	ErrDeviceCreateFailed        = NewBaseError(http.StatusInternalServerError, "DEVICE_CREATE_FAILED", "建立裝置失敗", "")
	ErrDeviceUpdateFailed        = NewBaseError(http.StatusInternalServerError, "DEVICE_UPDATE_FAILED", "更新裝置失敗", "")
	ErrDeviceOwnershipViolation  = NewBaseError(http.StatusForbidden, "DEVICE_OWNERSHIP_VIOLATION", "您沒有權限存取此裝置", "")
)
