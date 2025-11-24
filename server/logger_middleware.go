package main

import (
	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// loggerMiddleware adds a zap logger to the request context
func loggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add logger to context
		ctx := zapctx.WithLogger(c.Request.Context(), logger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
