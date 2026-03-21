package crypto

import (
	"context"
	"time"
)

const defaultCryptoTimeout = 30 * time.Second

func withDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, defaultCryptoTimeout)
}
