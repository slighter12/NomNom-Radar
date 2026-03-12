package handler

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"radar/internal/delivery/api/response"

	validatorpkg "github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type PaginationQueryParams struct {
	Page     int `query:"page" validate:"gte=1"`
	PageSize int `query:"page_size" validate:"gte=1"`
}

type LimitOffsetQueryParams struct {
	Limit  int `query:"limit" validate:"gte=1"`
	Offset int `query:"offset" validate:"gte=0"`
}

func NewPaginationQueryParams(defaultPage, defaultPageSize int) PaginationQueryParams {
	return PaginationQueryParams{
		Page:     defaultPage,
		PageSize: defaultPageSize,
	}
}

func NewLimitOffsetQueryParams(defaultLimit, defaultOffset int) LimitOffsetQueryParams {
	return LimitOffsetQueryParams{
		Limit:  defaultLimit,
		Offset: defaultOffset,
	}
}

func bindQueryParams(c echo.Context, params any) error {
	return (&echo.DefaultBinder{}).BindQueryParams(c, params)
}

func bindRequest(c echo.Context, req any, invalidMessage string) error {
	if err := c.Bind(req); err != nil {
		return response.BindingError(c, "INVALID_INPUT", invalidMessage)
	}

	return nil
}

func validateRequest(c echo.Context, req any) error {
	if err := c.Validate(req); err != nil {
		return response.BadRequest(c, "VALIDATION_ERROR", validationMessage(err, req))
	}

	return nil
}

func bindAndValidateRequest(c echo.Context, req any, invalidMessage string) error {
	if err := bindRequest(c, req, invalidMessage); err != nil {
		return err
	}

	return validateRequest(c, req)
}

func requireBoundPayload[T any](c echo.Context, payload *T) error {
	if payload == nil {
		return response.BadRequest(c, "VALIDATION_ERROR", "請檢查輸入內容")
	}

	return nil
}

func bindRequiredPayload[T any](c echo.Context, invalidMessage string) (*T, error) {
	var payload *T
	if err := bindRequest(c, &payload, invalidMessage); err != nil {
		return nil, err
	}
	if err := requireBoundPayload(c, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func bindMerchantIDPathParam(c echo.Context, invalidMessage string) (uuid.UUID, error) {
	return bindUUIDPathParam(c, "merchantId", invalidMessage)
}

func bindLocationIDPathParam(c echo.Context, invalidMessage string) (uuid.UUID, error) {
	return bindUUIDPathParam(c, "locationId", invalidMessage)
}

func bindMenuItemIDPathParam(c echo.Context, invalidMessage string) (uuid.UUID, error) {
	return bindUUIDPathParam(c, "menuItemId", invalidMessage)
}

func bindDeviceIDPathParam(c echo.Context, invalidMessage string) (uuid.UUID, error) {
	return bindUUIDPathParam(c, "deviceId", invalidMessage)
}

func bindUUIDPathParam(c echo.Context, paramName, invalidMessage string) (uuid.UUID, error) {
	value := strings.TrimSpace(c.Param(paramName))
	if value == "" {
		return uuid.Nil, response.BadRequest(c, "INVALID_ID", invalidMessage)
	}

	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, response.BadRequest(c, "INVALID_ID", invalidMessage)
	}

	return id, nil
}

func validatePaginationQueryParams(c echo.Context, params *PaginationQueryParams) error {
	if err := c.Validate(params); err != nil {
		if validationErrors, ok := errors.AsType[validatorpkg.ValidationErrors](err); ok {
			for _, validationErr := range validationErrors {
				switch validationErr.StructField() {
				case "Page":
					return response.BadRequest(c, "VALIDATION_ERROR", "page 必須為大於 0 的整數")
				case "PageSize":
					return response.BadRequest(c, "VALIDATION_ERROR", "page_size 必須為大於 0 的整數")
				}
			}
		}

		return response.BadRequest(c, "VALIDATION_ERROR", "Invalid pagination query input")
	}

	return nil
}

func validationMessage(err error, req any) string {
	if validationErrors, ok := errors.AsType[validatorpkg.ValidationErrors](err); ok {
		validationErr := validationErrors[0]
		fieldName := requestFieldName(req, validationErr)

		switch validationErr.Tag() {
		case "required":
			return fmt.Sprintf("%s is required", fieldName)
		case "oneof":
			return fmt.Sprintf("%s must be one of [%s]", fieldName, validationErr.Param())
		case "min", "gte":
			return fmt.Sprintf("%s must be greater than or equal to %s", fieldName, validationErr.Param())
		case "max", "lte":
			return fmt.Sprintf("%s must be less than or equal to %s", fieldName, validationErr.Param())
		case "email":
			return fmt.Sprintf("%s must be a valid email address", fieldName)
		}

		return fmt.Sprintf("%s is invalid", fieldName)
	}

	return "請檢查輸入內容"
}

func requestFieldName(req any, validationErr validatorpkg.FieldError) string {
	field, ok := requestStructField(req, validationErr.StructNamespace())
	if !ok {
		return strings.ToLower(validationErr.Field())
	}

	for _, tagName := range []string{"json", "query", "param", "form"} {
		if tagValue := requestTagValue(&field, tagName); tagValue != "" {
			return tagValue
		}
	}

	return strings.ToLower(validationErr.Field())
}

func requestStructField(req any, structNamespace string) (reflect.StructField, bool) {
	reqType, ok := requestStructType(req)
	if !ok {
		return reflect.StructField{}, false
	}

	path := requestStructPath(reqType, structNamespace)
	if len(path) == 0 {
		return reflect.StructField{}, false
	}

	return requestNestedStructField(reqType, path)
}

func requestStructType(req any) (reflect.Type, bool) {
	reqType := reflect.TypeOf(req)
	for reqType != nil && reqType.Kind() == reflect.Pointer {
		reqType = reqType.Elem()
	}
	if reqType == nil || reqType.Kind() != reflect.Struct {
		return nil, false
	}

	return reqType, true
}

func requestStructPath(reqType reflect.Type, structNamespace string) []string {
	if structNamespace == "" {
		return nil
	}

	path := strings.Split(structNamespace, ".")
	if len(path) > 0 && path[0] == reqType.Name() {
		path = path[1:]
	}
	if len(path) == 0 {
		return nil
	}

	return path
}

func requestNestedStructField(reqType reflect.Type, path []string) (reflect.StructField, bool) {
	currentType := reqType
	var field reflect.StructField

	for _, segment := range path {
		if currentType.Kind() != reflect.Struct {
			return reflect.StructField{}, false
		}

		nextField, ok := currentType.FieldByName(requestFieldSegment(segment))
		if !ok {
			return reflect.StructField{}, false
		}

		field = nextField
		currentType = requestElementType(nextField.Type)
	}

	return field, true
}

func requestElementType(fieldType reflect.Type) reflect.Type {
	currentType := fieldType
	for currentType.Kind() == reflect.Pointer || currentType.Kind() == reflect.Slice || currentType.Kind() == reflect.Array {
		currentType = currentType.Elem()
	}

	return currentType
}

func requestFieldSegment(segment string) string {
	if before, _, ok := strings.Cut(segment, "["); ok {
		return before
	}

	return segment
}

func requestTagValue(field *reflect.StructField, tagName string) string {
	if field == nil {
		return ""
	}

	tagValue := field.Tag.Get(tagName)
	if tagValue == "" {
		return ""
	}

	name := strings.Split(tagValue, ",")[0]
	if name == "" || name == "-" {
		return ""
	}

	return name
}
