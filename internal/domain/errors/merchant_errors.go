package errors

import "net/http"

var (
	ErrMerchantAlreadyExists     = NewBaseError(http.StatusConflict, "MERCHANT_ALREADY_EXISTS", "此電子郵件已被註冊為商家", "")
	ErrBusinessLicenseExists     = NewBaseError(http.StatusConflict, "BUSINESS_LICENSE_ALREADY_EXISTS", "此營業登記已被註冊", "")
	ErrMerchantNotFound          = NewBaseError(http.StatusNotFound, "MERCHANT_NOT_FOUND", "找不到該商家", "")
	ErrMenuItemNotFound          = NewBaseError(http.StatusNotFound, "MENU_ITEM_NOT_FOUND", "找不到該菜單品項", "")
	ErrMenuItemCreateFailed      = NewBaseError(http.StatusInternalServerError, "MENU_ITEM_CREATE_FAILED", "建立菜單品項失敗", "")
	ErrMenuItemUpdateFailed      = NewBaseError(http.StatusInternalServerError, "MENU_ITEM_UPDATE_FAILED", "更新菜單品項失敗", "")
	ErrMenuItemOrderConflict     = NewBaseError(http.StatusConflict, "MENU_ITEM_ORDER_CONFLICT", "菜單排序衝突", "")
	ErrSubscriptionNotFound      = NewBaseError(http.StatusNotFound, "SUBSCRIPTION_NOT_FOUND", "找不到訂閱資料", "")
	ErrSubscriptionAlreadyExists = NewBaseError(
		http.StatusConflict,
		"SUBSCRIPTION_ALREADY_EXISTS",
		"訂閱已存在",
		"",
	)
	ErrSubscriptionCreateFailed  = NewBaseError(http.StatusInternalServerError, "SUBSCRIPTION_CREATE_FAILED", "建立訂閱失敗", "")
	ErrInvalidNotificationRadius = NewBaseError(
		http.StatusBadRequest,
		"INVALID_NOTIFICATION_RADIUS",
		"無效的通知半徑",
		"",
	)
	ErrInvalidQRCode            = NewBaseError(http.StatusBadRequest, "INVALID_QR_CODE", "無效的訂閱 QR code", "")
	ErrInvalidNotificationData  = NewBaseError(http.StatusBadRequest, "INVALID_NOTIFICATION_DATA", "通知資料無效", "")
	ErrNotificationNotFound     = NewBaseError(http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "找不到通知資料", "")
	ErrNotificationLogNotFound  = NewBaseError(http.StatusNotFound, "NOTIFICATION_LOG_NOT_FOUND", "找不到通知紀錄", "")
	ErrNotificationCreateFailed = NewBaseError(
		http.StatusInternalServerError,
		"NOTIFICATION_CREATE_FAILED",
		"建立通知失敗",
		"",
	)
	ErrNotificationLogCreateFailed = NewBaseError(
		http.StatusInternalServerError,
		"NOTIFICATION_LOG_CREATE_FAILED",
		"建立通知紀錄失敗",
		"",
	)
	ErrSelfSubscriptionNotAllowed = NewBaseError(http.StatusBadRequest, "SELF_SUBSCRIPTION_NOT_ALLOWED", "不可訂閱自己", "")
)
