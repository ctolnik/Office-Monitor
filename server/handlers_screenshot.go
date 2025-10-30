package main

import (
	"net/http"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// getScreenshotHandler returns a presigned URL for downloading a screenshot
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

	url, err := storageClient.GetPresignedURL(ctx, "screenshots", objectName)
	if err != nil {
		zapctx.Error(ctx, "Failed to generate presigned URL", zap.Error(err), zap.String("screenshot_id", screenshotID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get screenshot"})
		return
	}

	zapctx.Info(ctx, "Generated presigned URL for screenshot", 
		zap.String("screenshot_id", screenshotID),
		zap.String("object_name", objectName),
		zap.String("url", url))
	c.JSON(http.StatusOK, gin.H{"url": url})
}
