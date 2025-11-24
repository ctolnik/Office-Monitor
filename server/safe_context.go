package main

import (
	"context"

	"github.com/ctolnik/Office-Monitor/zapctx"
)

// withLogger ensures context has a logger, adding global logger if missing.
// This prevents zapctx panics when database methods are called from non-HTTP contexts
// (background jobs, tests, cache refreshes, etc).
func withLogger(ctx context.Context) context.Context {
	// Use defer/recover to safely check if context has logger
	hasLogger := false
	func() {
		defer func() {
			if recover() != nil {
				hasLogger = false
			}
		}()
		// Try to get logger - will panic if not found
		_ = zapctx.Logger(ctx)
		hasLogger = true
	}()

	if !hasLogger && logger != nil {
		// Add global logger as fallback
		return zapctx.WithLogger(ctx, logger)
	}

	return ctx
}
