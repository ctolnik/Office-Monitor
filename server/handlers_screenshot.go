package main

import (
	"fmt"
	"net/http"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// getScreenshotHandler returns screenshot file directly (proxy from MinIO)
func getScreenshotHandler(c *gin.Context) {
	ctx := c.Request.Context()
	screenshotID := c.Param("id")

	if screenshotID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Screenshot ID is required"})
		return
	}

	if storageClient == nil {
		zapctx.Error(ctx, "Storage client not initialized")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Storage service unavailable"})
		return
	}

	// Screenshot is stored in 'screenshots' bucket with name: COMPUTER_USERNAME_TIMESTAMP.jpg
	objectName := screenshotID + ".jpg"

	// Get object from MinIO and stream it directly
	object, err := storageClient.GetObject(ctx, "screenshots", objectName)
	if err != nil {
		zapctx.Error(ctx, "Failed to get screenshot from storage", zap.Error(err), zap.String("screenshot_id", screenshotID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get screenshot"})
		return
	}
	defer object.Close()

	// Get object info for content type and size
	stat, err := object.Stat()
	if err != nil {
		zapctx.Error(ctx, "Failed to stat screenshot", zap.Error(err), zap.String("screenshot_id", screenshotID))
		c.JSON(http.StatusNotFound, gin.H{"error": "Screenshot not found"})
		return
	}

	zapctx.Debug(ctx, "Serving screenshot", 
		zap.String("screenshot_id", screenshotID),
		zap.String("object_name", objectName),
		zap.Int64("size", stat.Size))

	// Set headers
	c.Header("Content-Type", "image/jpeg")
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size))
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 1 day

	// Stream the file
	c.DataFromReader(http.StatusOK, stat.Size, "image/jpeg", object, nil)
}
