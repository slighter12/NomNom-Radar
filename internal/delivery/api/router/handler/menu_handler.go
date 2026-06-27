package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
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
	CategoryID  string  `json:"category_id" validate:"required,uuid"`
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
	CategoryID  string  `json:"category_id" validate:"required,uuid"`
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

type ListMerchantMenuItemsQueryParams struct {
	PaginationQueryParams
	CategoryID  string `query:"category_id" validate:"omitempty,uuid"`
	IsAvailable *bool  `query:"is_available"`
	Keyword     string `query:"keyword"`
}

type ListPublicMerchantMenuItemsQueryParams struct {
	PaginationQueryParams
	CategoryID string `query:"category_id" validate:"omitempty,uuid"`
	Keyword    string `query:"keyword"`
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
		return response.InvalidToken(c)
	}

	input, err := h.parseListMerchantMenuItemsInput(c)
	if err != nil {
		return err
	}

	result, err := h.menuUC.ListMerchantMenuItems(c.Request().Context(), merchantID, input)
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, result)
}

// GetPublicMerchantMenu returns the consumer-facing menu for a merchant.
// The route is intentionally mounted behind the user role and only exposes available items.
func (h *MenuHandler) GetPublicMerchantMenu(c echo.Context) error {
	merchantID, err := h.parseMerchantID(c)
	if err != nil {
		return err
	}

	input, err := h.parseListPublicMerchantMenuItemsInput(c)
	if err != nil {
		return err
	}

	result, err := h.menuUC.GetPublicMerchantMenu(c.Request().Context(), merchantID, input)
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *MenuHandler) CreateMenuItem(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.InvalidToken(c)
	}

	var req CreateMenuItemRequest
	if err := bindRequest(c, &req, "Invalid menu item input"); err != nil {
		return err
	}

	if err := h.validateCreateMenuItemRequest(c, &req); err != nil {
		return err
	}
	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		return validationFailedError("category_id must be a valid UUID")
	}

	item, err := h.menuUC.CreateMenuItem(c.Request().Context(), merchantID, &usecase.CreateMenuItemInput{
		Name:        req.Name,
		Description: req.Description,
		CategoryID:  categoryID,
		Price:       req.Price,
		Currency:    req.Currency,
		PrepMinutes: req.PrepMinutes,
		IsAvailable: req.IsAvailable,
		IsPopular:   req.IsPopular,
		ImageURL:    req.ImageURL,
		ExternalURL: req.ExternalURL,
	})
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusCreated, item)
}

func (h *MenuHandler) UpdateMenuItem(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.InvalidToken(c)
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
	categoryID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		return validationFailedError("category_id must be a valid UUID")
	}

	item, err := h.menuUC.UpdateMenuItem(c.Request().Context(), merchantID, itemID, &usecase.UpdateMenuItemInput{
		Name:        req.Name,
		Description: req.Description,
		CategoryID:  categoryID,
		Price:       req.Price,
		Currency:    req.Currency,
		PrepMinutes: req.PrepMinutes,
		IsAvailable: *req.IsAvailable,
		IsPopular:   *req.IsPopular,
		ImageURL:    req.ImageURL,
		ExternalURL: req.ExternalURL,
	})
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, item)
}

func (h *MenuHandler) UpdateMenuItemStatus(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.InvalidToken(c)
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
		return validationFailedError("is_available is required")
	}

	item, err := h.menuUC.UpdateMenuItemStatus(c.Request().Context(), merchantID, itemID, *req.IsAvailable)
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, item)
}

func (h *MenuHandler) ReorderMenuItems(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.InvalidToken(c)
	}

	var req ReorderMenuItemsRequest
	if err := bindAndValidateRequest(c, &req, "Invalid menu item reorder input"); err != nil {
		return err
	}

	result, err := h.menuUC.ReorderMenuItems(c.Request().Context(), merchantID, &usecase.ReorderMenuItemsInput{
		ItemIDs: req.ItemIDs,
	})
	if err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, result)
}

func (h *MenuHandler) DeleteMenuItem(c echo.Context) error {
	merchantID, ok := middleware.GetUserID(c)
	if !ok {
		return response.InvalidToken(c)
	}

	itemID, err := h.parseMenuItemID(c)
	if err != nil {
		return err
	}

	if err := h.menuUC.DeleteMenuItem(c.Request().Context(), merchantID, itemID); err != nil {
		return withSourceStack(err)
	}

	return response.Success(c, http.StatusOK, map[string]string{responseKeyMessage: "Menu item deleted"})
}

func (h *MenuHandler) parseMenuItemID(c echo.Context) (uuid.UUID, error) {
	return bindMenuItemIDPathParam(c, "Invalid menu item ID")
}

func (h *MenuHandler) parseMerchantID(c echo.Context) (uuid.UUID, error) {
	return bindMerchantIDPathParam(c, "Invalid merchant ID")
}

func (h *MenuHandler) parseListMerchantMenuItemsInput(c echo.Context) (*usecase.ListMerchantMenuItemsInput, error) {
	query := newListMerchantMenuItemsQueryParams()
	if err := bindQueryParams(c, &query, "Invalid menu item query input"); err != nil {
		return nil, err
	}

	normalizePaginationQueryParams(c, &query.PaginationQueryParams)
	if strings.TrimSpace(c.QueryParam("is_available")) == "" {
		query.IsAvailable = nil
	}

	if err := validatePaginationQueryParams(c, &query.PaginationQueryParams); err != nil {
		return nil, err
	}
	categoryID, err := parseOptionalUUIDQueryValue(c, "category_id", query.CategoryID)
	if err != nil {
		return nil, err
	}

	query.PageSize = min(query.PageSize, maxMenuItemsPageSize)

	return &usecase.ListMerchantMenuItemsInput{
		CategoryID:  categoryID,
		IsAvailable: query.IsAvailable,
		Keyword:     strings.TrimSpace(query.Keyword),
		Page:        query.Page,
		PageSize:    query.PageSize,
	}, nil
}

func (h *MenuHandler) parseListPublicMerchantMenuItemsInput(c echo.Context) (*usecase.ListMerchantMenuItemsInput, error) {
	query := newListPublicMerchantMenuItemsQueryParams()
	if err := bindQueryParams(c, &query, "Invalid menu item query input"); err != nil {
		return nil, err
	}

	normalizePaginationQueryParams(c, &query.PaginationQueryParams)

	if err := validatePaginationQueryParams(c, &query.PaginationQueryParams); err != nil {
		return nil, err
	}
	categoryID, err := parseOptionalUUIDQueryValue(c, "category_id", query.CategoryID)
	if err != nil {
		return nil, err
	}

	query.PageSize = min(query.PageSize, maxMenuItemsPageSize)

	return &usecase.ListMerchantMenuItemsInput{
		CategoryID: categoryID,
		Keyword:    strings.TrimSpace(query.Keyword),
		Page:       query.Page,
		PageSize:   query.PageSize,
	}, nil
}

func newListMerchantMenuItemsQueryParams() ListMerchantMenuItemsQueryParams {
	return ListMerchantMenuItemsQueryParams{
		PaginationQueryParams: NewPaginationQueryParams(defaultMenuItemsPage, defaultMenuItemsPageSize),
	}
}

func newListPublicMerchantMenuItemsQueryParams() ListPublicMerchantMenuItemsQueryParams {
	return ListPublicMerchantMenuItemsQueryParams{
		PaginationQueryParams: NewPaginationQueryParams(defaultMenuItemsPage, defaultMenuItemsPageSize),
	}
}

func normalizePaginationQueryParams(c echo.Context, params *PaginationQueryParams) {
	if strings.TrimSpace(c.QueryParam("page")) == "" {
		params.Page = defaultMenuItemsPage
	}

	if strings.TrimSpace(c.QueryParam("page_size")) == "" {
		params.PageSize = defaultMenuItemsPageSize
	}
}

func (h *MenuHandler) validateCreateMenuItemRequest(c echo.Context, req *CreateMenuItemRequest) error {
	return h.validateMenuItemRequest(c, req, &req.Name, &req.CategoryID, &req.Currency, req.ImageURL, req.ExternalURL)
}

func (h *MenuHandler) validateUpdateMenuItemRequest(c echo.Context, req *UpdateMenuItemRequest) error {
	return h.validateMenuItemRequest(c, req, &req.Name, &req.CategoryID, &req.Currency, req.ImageURL, req.ExternalURL)
}

func (h *MenuHandler) validateMenuItemRequest(
	c echo.Context,
	req any,
	name, categoryID, currency *string,
	imageURL, externalURL *string,
) error {
	normalizeMenuItemRequestFields(name, categoryID, currency)
	if err := validateRequest(c, req); err != nil {
		return err
	}

	return h.validateMenuItemRequestURLs(imageURL, externalURL)
}

func normalizeMenuItemRequestFields(name, categoryID, currency *string) {
	if name != nil {
		*name = strings.TrimSpace(*name)
	}
	if categoryID != nil {
		*categoryID = strings.TrimSpace(*categoryID)
	}
	if currency != nil {
		*currency = strings.TrimSpace(*currency)
	}
}

func (h *MenuHandler) validateMenuItemRequestURLs(imageURL, externalURL *string) error {
	if err := validateOptionalHTTPURL(imageURL); err != nil {
		return validationFailedError("image_url must be a valid http/https URL")
	}
	if err := validateOptionalHTTPURL(externalURL); err != nil {
		return validationFailedError("external_url must be a valid http/https URL")
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
		return errors.New("invalid URL format")
	}
	if parsedURL.Host == "" {
		return errors.New("URL host is required")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}

	return nil
}
