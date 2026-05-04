package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
)

const (
	capturedRequestBodyKey      = "captured_request_body_for_error_log"
	maxLoggedBodyBytes          = 16 * 1024
	maxLoggedStringLength       = 512
	redactedLogValue            = "[REDACTED]"
	binaryRedactedLogValue      = "[BINARY_REDACTED]"
	invalidJSONRedactedLogValue = "[INVALID_JSON_REDACTED]"
	truncatedLogSuffix          = "...[TRUNCATED]"
)

// CaptureRequestBodyForErrorLog keeps a bounded copy of JSON request bodies so
// the centralized error handler can log a sanitized request after binding.
func CaptureRequestBodyForErrorLog(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		if req.Body == nil || !requestMayHaveBody(req.Method) || !isJSONContentType(req.Header.Get(echo.HeaderContentType)) {
			return next(c)
		}

		originalBody := req.Body
		body, err := io.ReadAll(originalBody)
		_ = originalBody.Close()
		if err != nil {
			return fmt.Errorf("read request body for error log: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(body))

		if len(body) > 0 {
			c.Set(capturedRequestBodyKey, body)
		}

		return next(c)
	}
}

type sanitizedRequestLog struct {
	Method        string         `json:"method"`
	Path          string         `json:"path"`
	Query         map[string]any `json:"query,omitempty"`
	Headers       map[string]any `json:"headers,omitempty"`
	Body          any            `json:"body,omitempty"`
	BodyTruncated bool           `json:"body_truncated,omitempty"`
}

type responseErrorLog struct {
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (log sanitizedRequestLog) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.String("method", log.Method),
		slog.String("path", log.Path),
	}
	if len(log.Query) > 0 {
		attrs = append(attrs, slog.Any("query", log.Query))
	}
	if len(log.Headers) > 0 {
		attrs = append(attrs, slog.Any("headers", log.Headers))
	}
	if log.Body != nil {
		attrs = append(attrs, slog.Any("body", log.Body))
	}
	if log.BodyTruncated {
		attrs = append(attrs, slog.Bool("body_truncated", log.BodyTruncated))
	}

	return slog.GroupValue(attrs...)
}

func (log responseErrorLog) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Int("status", log.Status),
		slog.String("code", log.Code),
		slog.String("message", log.Message),
	}
	if log.Details != nil {
		attrs = append(attrs, slog.Any("details", log.Details))
	}

	return slog.GroupValue(attrs...)
}

func buildSanitizedRequestLog(c echo.Context) sanitizedRequestLog {
	req := c.Request()
	body, bodyTruncated := sanitizedBody(c)

	return sanitizedRequestLog{
		Method:        req.Method,
		Path:          req.URL.Path,
		Query:         sanitizeQuery(req),
		Headers:       sanitizeHeaders(req.Header),
		Body:          body,
		BodyTruncated: bodyTruncated,
	}
}

func sanitizedBody(c echo.Context) (any, bool) {
	raw, ok := c.Get(capturedRequestBodyKey).([]byte)
	if !ok || len(raw) == 0 {
		return nil, false
	}

	truncated := false
	if len(raw) > maxLoggedBodyBytes {
		raw = raw[:maxLoggedBodyBytes]
		truncated = true
	}

	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return invalidJSONRedactedLogValue, truncated
	}

	return sanitizeLogValue(parsed, ""), truncated
}

func sanitizeQuery(req *http.Request) map[string]any {
	values := req.URL.Query()
	if len(values) == 0 {
		return nil
	}

	sanitized := make(map[string]any, len(values))
	for key, vals := range values {
		if isSensitiveKey(key) {
			sanitized[key] = redactedLogValue

			continue
		}
		if len(vals) == 1 {
			sanitized[key] = sanitizeString(vals[0])

			continue
		}

		cleanValues := make([]any, 0, len(vals))
		for _, val := range vals {
			cleanValues = append(cleanValues, sanitizeString(val))
		}
		sanitized[key] = cleanValues
	}

	return sanitized
}

func sanitizeHeaders(headers http.Header) map[string]any {
	if len(headers) == 0 {
		return nil
	}

	sanitized := map[string]any{}
	for key, values := range headers {
		normalizedKey := http.CanonicalHeaderKey(key)
		switch strings.ToLower(normalizedKey) {
		case "content-type", "content-length", "user-agent", "x-request-id":
			sanitized[normalizedKey] = sanitizeHeaderValues(values)
		default:
			if isSensitiveKey(normalizedKey) {
				sanitized[normalizedKey] = redactedLogValue
			}
		}
	}
	if len(sanitized) == 0 {
		return nil
	}

	return sanitized
}

func sanitizeHeaderValues(values []string) any {
	if len(values) == 1 {
		return sanitizeString(values[0])
	}

	sanitized := make([]any, 0, len(values))
	for _, value := range values {
		sanitized = append(sanitized, sanitizeString(value))
	}

	return sanitized
}

func sanitizeLogValue(value any, key string) any {
	if isSensitiveKey(key) {
		return redactedLogValue
	}

	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			sanitized[childKey] = sanitizeLogValue(childValue, childKey)
		}

		return sanitized
	case []any:
		sanitized := make([]any, 0, len(typed))
		for _, childValue := range typed {
			sanitized = append(sanitized, sanitizeLogValue(childValue, key))
		}

		return sanitized
	case string:
		return sanitizeString(typed)
	default:
		return typed
	}
}

func sanitizeString(value string) string {
	if !utf8.ValidString(value) {
		return binaryRedactedLogValue
	}
	if len(value) > maxLoggedStringLength {
		i := maxLoggedStringLength
		for i > 0 && !utf8.RuneStart(value[i]) {
			i--
		}

		return value[:i] + truncatedLogSuffix
	}

	return value
}

func sanitizeFreeTextLogValue(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return sanitizeString(value)
	}

	sanitizedFields := make([]string, 0, len(fields))
	redactNext := false
	for _, field := range fields {
		if redactNext {
			sanitizedFields = append(sanitizedFields, replaceTokenCore(field, redactedLogValue))
			redactNext = false

			continue
		}

		prefix, core, suffix := splitTokenPunctuation(field)
		if strings.EqualFold(core, "Bearer") {
			sanitizedFields = append(sanitizedFields, prefix+"Bearer "+redactedLogValue+suffix)
			redactNext = true

			continue
		}

		if key, separator, val, ok := splitAssignment(core); ok && isSensitiveKey(key) {
			sanitizedFields = append(sanitizedFields, prefix+key+separator+redactedLogValue+suffix)
			if strings.EqualFold(stripQuotes(val), "Bearer") {
				redactNext = true
			}

			continue
		}

		if looksLikeEmail(core) {
			sanitizedFields = append(sanitizedFields, prefix+redactedLogValue+suffix)

			continue
		}

		sanitizedFields = append(sanitizedFields, field)
	}

	sanitized := strings.Join(sanitizedFields, " ")

	return sanitizeString(sanitized)
}

func splitTokenPunctuation(value string) (string, string, string) {
	start := 0
	for start < len(value) && isTokenPunctuation(value[start]) {
		start++
	}

	end := len(value)
	for end > start && isTokenPunctuation(value[end-1]) {
		end--
	}

	return value[:start], value[start:end], value[end:]
}

func replaceTokenCore(value string, replacement string) string {
	prefix, _, suffix := splitTokenPunctuation(value)

	return prefix + replacement + suffix
}

func isTokenPunctuation(ch byte) bool {
	return strings.ContainsRune(`"'()[]{}<>,;.`, rune(ch))
}

func splitAssignment(value string) (string, string, string, bool) {
	index := strings.IndexAny(value, "=:")
	if index <= 0 || index == len(value)-1 {
		return "", "", "", false
	}

	return value[:index], value[index : index+1], value[index+1:], true
}

func stripQuotes(value string) string {
	return strings.Trim(value, `"'`)
}

func looksLikeEmail(value string) bool {
	at := strings.IndexByte(value, '@')
	if at <= 0 || at != strings.LastIndexByte(value, '@') || at == len(value)-1 {
		return false
	}

	domain := value[at+1:]
	dot := strings.LastIndexByte(domain, '.')

	return dot > 0 && dot < len(domain)-1
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	if strings.Contains(normalized, "token") {
		return true
	}

	switch normalized {
	case "authorization",
		"cookie",
		"password",
		"passwd",
		"pwd",
		"token",
		"access_token",
		"refresh_token",
		"id_token",
		"secret",
		"credential",
		"credentials",
		"email",
		"phone",
		"address",
		"business_license",
		"store_name",
		"latitude",
		"longitude",
		"lat",
		"lon":
		return true
	default:
		return false
	}
}

func requestMayHaveBody(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isJSONContentType(contentType string) bool {
	if contentType == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.Contains(strings.ToLower(contentType), "json")
	}

	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}
