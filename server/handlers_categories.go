package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ctolnik/Office-Monitor/server/database"
	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// getAppCategoriesHandler returns all application categories
func getAppCategoriesHandler(c *gin.Context) {
	ctx := c.Request.Context()

	category := c.Query("category")
	search := c.Query("search")
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	zapctx.Debug(ctx, "Fetching application categories",
		zap.String("category", category),
		zap.String("search", search),
		zap.Bool("active_only", activeOnly))

	categories, err := db.GetApplicationCategories(ctx, category, search, activeOnly)
	if err != nil {
		zapctx.Warn(ctx, "Failed to get categories (table might not exist yet), returning empty list", zap.Error(err))
		// Return empty array instead of 500 if table doesn't exist
		c.JSON(http.StatusOK, gin.H{
			"data":  []database.ApplicationCategory{},
			"total": 0,
		})
		return
	}

	zapctx.Info(ctx, "Application categories fetched",
		zap.Int("count", len(categories)))

	c.JSON(http.StatusOK, gin.H{
		"data":  categories,
		"total": len(categories),
	})
}

// createAppCategoryHandler creates a new application category
func createAppCategoryHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var cat database.ApplicationCategory
	if err := c.ShouldBindJSON(&cat); err != nil {
		zapctx.Warn(ctx, "Invalid category request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Set created_by and updated_by to "admin" for now
	// TODO: Get from auth context when auth is implemented
	cat.CreatedBy = "admin"
	cat.UpdatedBy = "admin"

	if err := db.CreateApplicationCategory(ctx, cat); err != nil {
		zapctx.Error(ctx, "Failed to create category",
			zap.Error(err),
			zap.String("process_name", cat.ProcessName))

		// Check if it's a duplicate error
		if strings.Contains(err.Error(), "already exists") {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	zapctx.Info(ctx, "Application category created",
		zap.String("process_name", cat.ProcessName),
		zap.String("category", cat.Category))

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"message": "Category created successfully",
	})
}

// updateAppCategoryHandler updates an existing application category
func updateAppCategoryHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	var cat database.ApplicationCategory
	if err := c.ShouldBindJSON(&cat); err != nil {
		zapctx.Warn(ctx, "Invalid update request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Set updated_by
	cat.UpdatedBy = "admin" // TODO: Get from auth context

	if err := db.UpdateApplicationCategory(ctx, id, cat); err != nil {
		zapctx.Error(ctx, "Failed to update category",
			zap.Error(err),
			zap.String("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update category"})
		return
	}

	zapctx.Info(ctx, "Application category updated",
		zap.String("id", id),
		zap.String("process_name", cat.ProcessName))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Category updated successfully",
	})
}

// deleteAppCategoryHandler deletes (soft delete) an application category
func deleteAppCategoryHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	if err := db.DeleteApplicationCategory(ctx, id); err != nil {
		zapctx.Error(ctx, "Failed to delete category",
			zap.Error(err),
			zap.String("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
		return
	}

	zapctx.Info(ctx, "Application category deleted", zap.String("id", id))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Category deleted successfully",
	})
}

// bulkUpdateAppCategoriesHandler performs bulk update of categories
func bulkUpdateAppCategoriesHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		IDs      []string `json:"ids" binding:"required"`
		Category string   `json:"category" binding:"required,oneof=productive unproductive neutral communication system"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		zapctx.Warn(ctx, "Invalid bulk update request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updatedBy := "admin" // TODO: Get from auth context
	count, err := db.BulkUpdateCategories(ctx, req.IDs, req.Category, updatedBy)
	if err != nil {
		zapctx.Error(ctx, "Failed to bulk update categories",
			zap.Error(err),
			zap.Int("count", len(req.IDs)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bulk update"})
		return
	}

	zapctx.Info(ctx, "Categories bulk updated",
		zap.Int("count", count),
		zap.String("category", req.Category))

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"updated": count,
	})
}

// exportAppCategoriesHandler exports categories to JSON or CSV
func exportAppCategoriesHandler(c *gin.Context) {
	ctx := c.Request.Context()
	format := c.Query("format")

	if format != "json" && format != "csv" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Use 'json' or 'csv'"})
		return
	}

	// Get all active categories
	categories, err := db.GetApplicationCategories(ctx, "", "", true)
	if err != nil {
		zapctx.Error(ctx, "Failed to get categories for export", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export"})
		return
	}

	zapctx.Info(ctx, "Exporting categories",
		zap.String("format", format),
		zap.Int("count", len(categories)))

	if format == "json" {
		c.Header("Content-Disposition", "attachment; filename=app_categories.json")
		c.Header("Content-Type", "application/json")
		c.JSON(http.StatusOK, categories)
		return
	}

	// CSV export
	c.Header("Content-Disposition", "attachment; filename=app_categories.csv")
	c.Header("Content-Type", "text/csv")

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	// Write header
	if err := w.Write([]string{"process_name", "process_pattern", "category"}); err != nil {
		zapctx.Error(ctx, "Failed to write CSV header", zap.Error(err))
		return
	}

	// Write rows
	for _, cat := range categories {
		if err := w.Write([]string{cat.ProcessName, cat.ProcessPattern, cat.Category}); err != nil {
			zapctx.Error(ctx, "Failed to write CSV row", zap.Error(err))
			return
		}
	}
}

// importAppCategoriesHandler imports categories from JSON or CSV file
func importAppCategoriesHandler(c *gin.Context) {
	ctx := c.Request.Context()

	file, err := c.FormFile("file")
	if err != nil {
		zapctx.Warn(ctx, "No file uploaded", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Check file size (max 10MB)
	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large (max 10MB)"})
		return
	}

	// Open file
	f, err := file.Open()
	if err != nil {
		zapctx.Error(ctx, "Failed to open uploaded file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}
	defer f.Close()

	// Determine format by extension
	filename := strings.ToLower(file.Filename)
	isJSON := strings.HasSuffix(filename, ".json")
	isCSV := strings.HasSuffix(filename, ".csv")

	if !isJSON && !isCSV {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file format. Use .json or .csv"})
		return
	}

	var categories []database.ApplicationCategory
	var importErrors []database.ImportError

	if isJSON {
		decoder := json.NewDecoder(f)
		if err := decoder.Decode(&categories); err != nil {
			zapctx.Error(ctx, "Failed to parse JSON", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
			return
		}
	} else {
		// CSV parsing
		reader := csv.NewReader(f)
		records, err := reader.ReadAll()
		if err != nil {
			zapctx.Error(ctx, "Failed to parse CSV", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid CSV format"})
			return
		}

		// Skip header if present
		startIdx := 0
		if len(records) > 0 && records[0][0] == "process_name" {
			startIdx = 1
		}

		for i := startIdx; i < len(records); i++ {
			if len(records[i]) < 3 {
				importErrors = append(importErrors, database.ImportError{
					Line:        i + 1,
					ProcessName: "",
					Error:       "Insufficient columns",
				})
				continue
			}

			categories = append(categories, database.ApplicationCategory{
				ProcessName:    records[i][0],
				ProcessPattern: records[i][1],
				Category:       records[i][2],
				CreatedBy:      "admin",
				UpdatedBy:      "admin",
			})
		}
	}

	// Import categories
	imported := 0
	skipped := 0

	for i, cat := range categories {
		// Validate category
		validCategories := map[string]bool{
			"productive":    true,
			"unproductive":  true,
			"neutral":       true,
			"communication": true,
			"system":        true,
		}

		if !validCategories[cat.Category] {
			importErrors = append(importErrors, database.ImportError{
				Line:        i + 1,
				ProcessName: cat.ProcessName,
				Error:       fmt.Sprintf("Invalid category: %s", cat.Category),
			})
			skipped++
			continue
		}

		// Set metadata
		cat.CreatedBy = "admin"
		cat.UpdatedBy = "admin"

		// Try to create
		if err := db.CreateApplicationCategory(ctx, cat); err != nil {
			if strings.Contains(err.Error(), "already exists") {
				importErrors = append(importErrors, database.ImportError{
					Line:        i + 1,
					ProcessName: cat.ProcessName,
					Error:       "Duplicate entry",
				})
				skipped++
			} else {
				importErrors = append(importErrors, database.ImportError{
					Line:        i + 1,
					ProcessName: cat.ProcessName,
					Error:       err.Error(),
				})
				skipped++
			}
			continue
		}

		imported++
	}

	zapctx.Info(ctx, "Import completed",
		zap.Int("imported", imported),
		zap.Int("skipped", skipped),
		zap.Int("errors", len(importErrors)))

	c.JSON(http.StatusOK, database.ImportResult{
		Success:   imported,
		Failed:    skipped,
		TotalRows: imported + skipped,
		Errors:    importErrors,
	})
}
