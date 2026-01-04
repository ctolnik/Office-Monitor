package main

import (
	"net/http"

	"github.com/ctolnik/Office-Monitor/server/database"
	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func getCategoryTypesHandler(c *gin.Context) {
	ctx := c.Request.Context()
	activeOnly := c.DefaultQuery("active_only", "true") == "true"

	cats, err := db.GetCategoryTypes(ctx, activeOnly)
	if err != nil {
		zapctx.Error(ctx, "Failed to get category types", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cats, "total": len(cats)})
}

func createCategoryTypeHandler(c *gin.Context) {
	ctx := c.Request.Context()
	var req struct {
		Key       string `json:"key" binding:"required"`
		Name      string `json:"name" binding:"required"`
		Color     string `json:"color"`
		SortOrder int32  `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	created, err := db.CreateCategoryType(ctx, database.CategoryType{
		Key:       req.Key,
		Name:      req.Name,
		Color:     req.Color,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		zapctx.Error(ctx, "Failed to create category type", zap.Error(err), zap.String("key", req.Key))
		// Keep generic for now
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func updateCategoryTypeHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID required"})
		return
	}

	var req database.CategoryTypeUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updated, err := db.UpdateCategoryType(ctx, id, req)
	if err != nil {
		zapctx.Error(ctx, "Failed to update category type", zap.Error(err), zap.String("id", id))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func deleteCategoryTypeHandler(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID required"})
		return
	}

	if err := db.DeleteCategoryType(ctx, id); err != nil {
		zapctx.Error(ctx, "Failed to delete category type", zap.Error(err), zap.String("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
