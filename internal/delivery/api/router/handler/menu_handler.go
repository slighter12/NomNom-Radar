package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/response"
	"radar/internal/usecase"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/fx"
)

type MenuHandlerParams struct {
	fx.In

	MenuUC usecase.MenuUsecase
	Logger *slog.Logger
}

type MenuHandler struct {
	menuUC usecase.MenuUsecase
	logger *slog.Logger
}

type CreateMenuItemRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description"`
	Category    string  `json:"category" validate:"required,oneof=main snack drink dessert"`
	Price       int     `json:"price" validate:"gte=0"`
	Currency    string  `json:"currency" validate:"required"`
	PrepMinutes int     `json:"prep_minutes" validate:"gt=0"`
	IsAvailable *bool   `json:"is_available,omitempty"`
	IsPopular   *bool   `json:"is_popular,omitempty"`
	ImageURL    *string `json:"image_url"`
	ExternalURL *string `json:"external_url"`
}

type UpdateMenuItemRequest struct {
	Name        string  `json:"name" validate:"required"`
	Description *string `json:"description"`
	Category    string  `json:"category" validate:"required,oneof=main snack drink dessert"`
	Price       int     `json:"price" validate:"gte=0"`
	Currency    string  `json:"currency" validate:"required"`
	PrepMinutes int     `json:"prep_minutes" validate:"gt=0"`
	IsAvailable *bool   `json:"is_available" validate:"required"`
	IsPopular   *bool   `json:"is_popular" validate:"required"`
	ImageURL    *string `json:"image_url"`
	ExternalURL *string `json:"external_url"`
}

type UpdateMenuItemStatusRequest struct {
	IsAvailable *bool `json:"is_available"`
}

type ReorderMenuItemsRequest struct {
	ItemIDs []uuid.UUID `json:"item_ids" validate:"required,min=1"`
}

const (
	defaultMenuItemsPage     = 1
	defaultMenuItemsPageSize = 20
	maxMenuItemsPageSize     = 100
)

var errInvalidOptionalHTTPURL = errors.New("invalid optional http url")

type ListMerchantMenuItemsQueryParams struct {
	PaginationQueryParams
	Category    string `query:"category"`
	IsAvailable *bool  `query:"is_available"`
	Keyword     string `query:"keyword"`
}

func NewMenuHandler(params MenuHandlerParams) *MenuHandler {
	return &MenuHandler{
		menuUC: params.MenuUC,
		logger: params.Logger,
	}
}

func (h *MenuHandler) GetMerchantMenuItems(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	input, err := h.parseListMerchantMenuItemsInput(c)
	if err != nil {
		return err
	}

	result, err := h.menuUC.ListMerchantMenuItems(c.Request().Context(), merchantID, input)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *MenuHandler) CreateMenuItem(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	var req CreateMenuItemRequest
	if err := bindRequest(c, &req, "Invalid menu item input"); err != nil {
		return err
	}

	if err := h.validateCreateMenuItemRequest(c, &req); err != nil {
		return err
	}

	item, err := h.menuUC.CreateMenuItem(c.Request().Context(), merchantID, &usecase.CreateMenuItemInput{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Price:       req.Price,
		Currency:    req.Currency,
		PrepMinutes: req.PrepMinutes,
		IsAvailable: req.IsAvailable,
		IsPopular:   req.IsPopular,
		ImageURL:    req.ImageURL,
		ExternalURL: req.ExternalURL,
	})
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusCreated, item)
}

func (h *MenuHandler) UpdateMenuItem(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	itemID, err := h.parseMenuItemID(c)
	if err != nil {
		return err
	}

	var req UpdateMenuItemRequest
	if err := bindRequest(c, &req, "Invalid menu item input"); err != nil {
		return err
	}

	if err := h.validateUpdateMenuItemRequest(c, &req); err != nil {
		return err
	}

	item, err := h.menuUC.UpdateMenuItem(c.Request().Context(), merchantID, itemID, &usecase.UpdateMenuItemInput{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Price:       req.Price,
		Currency:    req.Currency,
		PrepMinutes: req.PrepMinutes,
		IsAvailable: *req.IsAvailable,
		IsPopular:   *req.IsPopular,
		ImageURL:    req.ImageURL,
		ExternalURL: req.ExternalURL,
	})
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, item)
}

func (h *MenuHandler) UpdateMenuItemStatus(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	itemID, err := h.parseMenuItemID(c)
	if err != nil {
		return err
	}

	var req UpdateMenuItemStatusRequest
	if err := bindRequest(c, &req, "Invalid menu item status input"); err != nil {
		return err
	}
	if req.IsAvailable == nil {
		return response.BadRequest(c, "VALIDATION_ERROR", "請提供 is_available")
	}

	item, err := h.menuUC.UpdateMenuItemStatus(c.Request().Context(), merchantID, itemID, *req.IsAvailable)
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, item)
}

func (h *MenuHandler) ReorderMenuItems(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	var req ReorderMenuItemsRequest
	if err := bindAndValidateRequest(c, &req, "Invalid menu item reorder input"); err != nil {
		return err
	}

	result, err := h.menuUC.ReorderMenuItems(c.Request().Context(), merchantID, &usecase.ReorderMenuItemsInput{
		ItemIDs: req.ItemIDs,
	})
	if err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *MenuHandler) DeleteMenuItem(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.Unauthorized(c, "INVALID_TOKEN", "Invalid user ID in token")
	}

	itemID, err := h.parseMenuItemID(c)
	if err != nil {
		return err
	}

	if err := h.menuUC.DeleteMenuItem(c.Request().Context(), merchantID, itemID); err != nil {
		return response.HandleAppError(c, err)
	}

	return response.Success(c, http.StatusOK, map[string]string{"message": "Menu item deleted"})
}

func (h *MenuHandler) parseMenuItemID(c echo.Context) (uuid.UUID, error) {
	return bindIDPathParam(c, "Invalid menu item ID")
}

func (h *MenuHandler) parseListMerchantMenuItemsInput(c echo.Context) (*usecase.ListMerchantMenuItemsInput, error) {
	query := newListMerchantMenuItemsQueryParams()
	if err := bindQueryParams(c, &query); err != nil {
		return nil, h.handleListMerchantMenuItemsQueryBindingError(c)
	}

	h.normalizeListMerchantMenuItemsQueryParams(c, &query)

	if err := validatePaginationQueryParams(c, &query.PaginationQueryParams); err != nil {
		return nil, err
	}

	query.PageSize = min(query.PageSize, maxMenuItemsPageSize)

	return &usecase.ListMerchantMenuItemsInput{
		Category:    strings.TrimSpace(query.Category),
		IsAvailable: query.IsAvailable,
		Keyword:     strings.TrimSpace(query.Keyword),
		Page:        query.Page,
		PageSize:    query.PageSize,
	}, nil
}

func newListMerchantMenuItemsQueryParams() ListMerchantMenuItemsQueryParams {
	return ListMerchantMenuItemsQueryParams{
		PaginationQueryParams: NewPaginationQueryParams(defaultMenuItemsPage, defaultMenuItemsPageSize),
	}
}

func (h *MenuHandler) normalizeListMerchantMenuItemsQueryParams(c echo.Context, query *ListMerchantMenuItemsQueryParams) {
	if strings.TrimSpace(c.QueryParam("page")) == "" {
		query.Page = defaultMenuItemsPage
	}

	if strings.TrimSpace(c.QueryParam("page_size")) == "" {
		query.PageSize = defaultMenuItemsPageSize
	}

	if strings.TrimSpace(c.QueryParam("is_available")) == "" {
		query.IsAvailable = nil
	}
}

func (h *MenuHandler) handleListMerchantMenuItemsQueryBindingError(c echo.Context) error {
	if pageValue := strings.TrimSpace(c.QueryParam("page")); pageValue != "" {
		if _, err := strconv.Atoi(pageValue); err != nil {
			return response.BadRequest(c, "VALIDATION_ERROR", "page 必須為大於 0 的整數")
		}
	}

	if pageSizeValue := strings.TrimSpace(c.QueryParam("page_size")); pageSizeValue != "" {
		if _, err := strconv.Atoi(pageSizeValue); err != nil {
			return response.BadRequest(c, "VALIDATION_ERROR", "page_size 必須為大於 0 的整數")
		}
	}

	if isAvailableValue := strings.TrimSpace(c.QueryParam("is_available")); isAvailableValue != "" {
		if _, err := strconv.ParseBool(isAvailableValue); err != nil {
			return response.BadRequest(c, "VALIDATION_ERROR", "is_available 必須為布林值")
		}
	}

	return response.BindingError(c, "INVALID_INPUT", "Invalid menu item query input")
}

func (h *MenuHandler) validateCreateMenuItemRequest(c echo.Context, req *CreateMenuItemRequest) error {
	normalizeMenuItemRequestFields(&req.Name, &req.Category, &req.Currency)
	if err := validateRequest(c, req); err != nil {
		return err
	}

	return h.validateMenuItemRequestURLs(c, req.ImageURL, req.ExternalURL)
}

func (h *MenuHandler) validateUpdateMenuItemRequest(c echo.Context, req *UpdateMenuItemRequest) error {
	normalizeMenuItemRequestFields(&req.Name, &req.Category, &req.Currency)
	if err := validateRequest(c, req); err != nil {
		return err
	}

	return h.validateMenuItemRequestURLs(c, req.ImageURL, req.ExternalURL)
}

func normalizeMenuItemRequestFields(name, category, currency *string) {
	if name != nil {
		*name = strings.TrimSpace(*name)
	}
	if category != nil {
		*category = strings.TrimSpace(*category)
	}
	if currency != nil {
		*currency = strings.TrimSpace(*currency)
	}
}

func (h *MenuHandler) validateMenuItemRequestURLs(c echo.Context, imageURL, externalURL *string) error {
	if err := validateOptionalHTTPURL(imageURL); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", "image_url 必須為有效的 http/https URL")
	}
	if err := validateOptionalHTTPURL(externalURL); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", "external_url 必須為有效的 http/https URL")
	}

	return nil
}

func validateOptionalHTTPURL(rawURL *string) error {
	if rawURL == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*rawURL)
	if trimmed == "" {
		return nil
	}

	parsedURL, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return errInvalidOptionalHTTPURL
	}
	if parsedURL.Host == "" {
		return errInvalidOptionalHTTPURL
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errInvalidOptionalHTTPURL
	}

	return nil
}
