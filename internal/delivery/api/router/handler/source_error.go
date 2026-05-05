package handler

import "radar/internal/delivery/api/middleware"

func withSourceStack(err error) error {
	return middleware.WithSourceStack(err)
}
