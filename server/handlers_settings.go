package main

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// getGeneralSettingsHandler returns all general system settings
func getGeneralSettingsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	settings, err := db.GetSystemSettings(ctx)
	if err != nil {
		zapctx.Error(ctx, "Failed to get system settings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get settings"})
		return
	}

	// Structure settings for frontend
	response := gin.H{
		"company_name":                settings["company_name"],
		"company_logo_url":            settings["company_logo_url"],
		"timezone":                    settings["timezone"],
		"working_hours_start":         settings["working_hours_start"],
		"working_hours_end":           settings["working_hours_end"],
		"productivity_threshold":      parseFloatOrDefault(settings["productivity_threshold"], 70.0),
		"alert_on_usb_events":         parseBoolOrDefault(settings["alert_on_usb_events"], true),
		"alert_on_file_copy":          parseBoolOrDefault(settings["alert_on_file_copy"], true),
		"alert_on_low_productivity":   parseBoolOrDefault(settings["alert_on_low_productivity"], false),
		"screenshot_retention_days":   parseIntOrDefault(settings["screenshot_retention_days"], 30),
		"activity_retention_days":     parseIntOrDefault(settings["activity_retention_days"], 90),
		"max_idle_time_minutes":       parseIntOrDefault(settings["max_idle_time_minutes"], 15),
		"enable_keylogger":            parseBoolOrDefault(settings["enable_keylogger"], false),
		"enable_screenshots":          parseBoolOrDefault(settings["enable_screenshots"], true),
		"enable_usb_monitoring":       parseBoolOrDefault(settings["enable_usb_monitoring"], true),
		"enable_file_monitoring":      parseBoolOrDefault(settings["enable_file_monitoring"], true),
		"screenshot_interval_minutes": parseIntOrDefault(settings["screenshot_interval_minutes"], 5),
	}

	zapctx.Debug(ctx, "General settings retrieved")
	c.JSON(http.StatusOK, response)
}

// updateGeneralSettingsHandler updates general system settings
func updateGeneralSettingsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		zapctx.Warn(ctx, "Invalid settings update request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Convert all values to strings for storage
	settings := make(map[string]string)
	for key, value := range req {
		switch v := value.(type) {
		case string:
			settings[key] = v
		case float64:
			settings[key] = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			settings[key] = strconv.FormatBool(v)
		case int:
			settings[key] = strconv.Itoa(v)
		default:
			zapctx.Warn(ctx, "Unknown setting value type", 
				zap.String("key", key),
				zap.Any("value", value))
			continue
		}
	}

	updatedBy := "admin" // TODO: Get from auth context
	if err := db.UpdateMultipleSettings(ctx, settings, updatedBy); err != nil {
		zapctx.Error(ctx, "Failed to update settings", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	// Invalidate dashboard cache when settings change
	if dashCache != nil {
		dashCache.Invalidate()
		zapctx.Debug(ctx, "Dashboard cache invalidated after settings update")
	}

	zapctx.Info(ctx, "System settings updated",
		zap.Int("count", len(settings)),
		zap.String("updated_by", updatedBy))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Settings updated successfully",
	})
}

// uploadLogoHandler handles company logo upload
func uploadLogoHandler(c *gin.Context) {
	ctx := c.Request.Context()

	file, err := c.FormFile("logo")
	if err != nil {
		zapctx.Warn(ctx, "No logo file uploaded", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Validate file type
	ext := filepath.Ext(file.Filename)
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".svg" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only PNG, JPG, JPEG, SVG are allowed"})
		return
	}

	// Validate file size (max 2MB)
	if file.Size > 2*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 2MB)"})
		return
	}

	// Generate unique filename
	timestamp := time.Now().Unix()
	filename := filepath.Join("logos", strconv.FormatInt(timestamp, 10)+ext)

	// Save file to MinIO/S3 storage
	f, err := file.Open()
	if err != nil {
		zapctx.Error(ctx, "Failed to open uploaded file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process file"})
		return
	}
	defer f.Close()

	// Upload to storage (if storage client is available)
	if storageClient != nil {
		// Read file data
		fileData, err := io.ReadAll(f)
		if err != nil {
			zapctx.Error(ctx, "Failed to read uploaded file", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process file"})
			return
		}

		objectName, err := storageClient.UploadScreenshot(ctx, filename, fileData)
		if err != nil {
			zapctx.Error(ctx, "Failed to upload logo to storage", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
			return
		}

		// Get presigned URL for logo (use screenshots bucket for logos)
		logoURL, err := storageClient.GetPresignedURL(ctx, "screenshots", objectName)
		if err != nil {
			zapctx.Error(ctx, "Failed to get logo URL", zap.Error(err))
			// Fallback to object name
			logoURL = "/storage/" + objectName
		}

		// Update company_logo_url setting
		updatedBy := "admin" // TODO: Get from auth context
		if err := db.UpdateSystemSetting(ctx, "company_logo_url", logoURL, updatedBy); err != nil {
			zapctx.Error(ctx, "Failed to update logo URL setting", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save logo URL"})
			return
		}

		zapctx.Info(ctx, "Logo uploaded successfully",
			zap.String("filename", filename),
			zap.String("url", logoURL))

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"url":    logoURL,
		})
		return
	}

	// If no storage client, save to local filesystem
	localPath := filepath.Join("./web/static/uploads", filename)
	if err := c.SaveUploadedFile(file, localPath); err != nil {
		zapctx.Error(ctx, "Failed to save logo locally", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	logoURL := "/static/uploads/" + filename

	// Update setting
	updatedBy := "admin"
	if err := db.UpdateSystemSetting(ctx, "company_logo_url", logoURL, updatedBy); err != nil {
		zapctx.Error(ctx, "Failed to update logo URL setting", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save logo URL"})
		return
	}

	zapctx.Info(ctx, "Logo saved locally",
		zap.String("path", localPath),
		zap.String("url", logoURL))

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"url":    logoURL,
	})
}

// Helper functions to parse settings with defaults

func parseFloatOrDefault(s string, defaultVal float64) float64 {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseBoolOrDefault(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.ParseBool(s)
	if err != nil {
		return defaultVal
	}
	return val
}

func parseIntOrDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}
