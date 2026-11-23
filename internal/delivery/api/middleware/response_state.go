package middleware

import (
	"github.com/labstack/echo/v5"
)

// responseState reports the status and committed flag recorded by Echo's
// response wrapper. Echo v5 exposes the response as an http.ResponseWriter, so
// the wrapper must be unwrapped to read them.
func responseState(c *echo.Context) (status int, committed bool) {
	resp, err := echo.UnwrapResponse(c.Response())
	if err != nil {
		return 0, false
	}

	return resp.Status, resp.Committed
}

func responseCommitted(c *echo.Context) bool {
	_, committed := responseState(c)

	return committed
}
