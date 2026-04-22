package main

import (
	"net/http"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// apiKeyMiddleware returns a Gin middleware that validates the `X-API-Key`
// request header against the configured key. When `enforce` is false the
// middleware only emits a warning on mismatch, enabling staged rollout of
// authentication without breaking existing agents. When `enforce` is true
// requests without a valid key are rejected with HTTP 401.
//
// An empty `apiKey` disables the middleware: no header checks and no warnings
// are emitted. This keeps local development and bootstrap scenarios simple.
func apiKeyMiddleware(apiKey string, enforce bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}

		got := c.GetHeader("X-API-Key")
		if got == apiKey {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		if got == "" {
			zapctx.Warn(ctx, "API key missing",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
				zap.Bool("enforce", enforce),
			)
		} else {
			zapctx.Warn(ctx, "API key mismatch",
				zap.String("path", c.Request.URL.Path),
				zap.String("client_ip", c.ClientIP()),
				zap.Bool("enforce", enforce),
			)
		}

		if enforce {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
