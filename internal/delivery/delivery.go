package delivery

import (
	"context"
)

type Delivery interface {
	Serve(ctx context.Context) error
}
