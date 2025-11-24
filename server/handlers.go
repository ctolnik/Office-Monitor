package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ctolnik/Office-Monitor/server/database"
	"github.com/ctolnik/Office-Monitor/zapctx"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ========== Agents Management Handlers ==========

func getAgentsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	agents, err := db.GetAgents(ctx)
	if err != nil {
		zapctx.Error(ctx, "Failed to get agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get agents"})
		return
	}
	c.JSON(http.StatusOK, agents)
}

func getAgentConfigHandler(c *gin.Context) {
	ctx := c.Request.Context()
	computerName := c.Param("computer_name")

	config, err := db.GetAgentConfig(ctx, computerName)
	if err != nil {
		zapctx.Error(ctx, "Failed to get agent config", zap.Error(err), zap.String("computer_name", computerName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get configuration"})
		return
	}

	c.JSON(http.StatusOK, config)
}

func updateAgentConfigHandler(c *gin.Context) {
	ctx := c.Request.Context()
	computerName := c.Param("computer_name")

	var config database.ConfigUpdate
	if err := c.ShouldBindJSON(&config); err != nil {
		zapctx.Warn(ctx, "Invalid agent config request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := db.UpdateAgentConfig(ctx, computerName, config); err != nil {
		zapctx.Error(ctx, "Failed to update agent config", zap.Error(err), zap.String("computer_name", computerName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update configuration"})
		return
	}

	zapctx.Info(ctx, "Agent config updated", zap.String("computer_name", computerName))
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func deleteAgentHandler(c *gin.Context) {
	ctx := c.Request.Context()
	computerName := c.Param("computer_name")

	if err := db.DeleteAgent(ctx, computerName); err != nil {
		zapctx.Error(ctx, "Failed to delete agent", zap.Error(err), zap.String("computer_name", computerName))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent"})
		return
	}

	zapctx.Info(ctx, "Agent deleted", zap.String("computer_name", computerName))
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ========== Users Handlers ==========

// ========== Employees Management Handlers ==========

func getAllEmployeesHandler(c *gin.Context) {
	ctx := c.Request.Context()
	employees, err := db.GetAllEmployees(ctx)
	if err != nil {
		zapctx.Error(ctx, "Failed to get employees", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get employees"})
		return
	}
	c.JSON(http.StatusOK, employees)
}

func createEmployeeHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var employee database.EmployeeFull
	if err := c.ShouldBindJSON(&employee); err != nil {
		zapctx.Warn(ctx, "Invalid employee request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := db.CreateEmployee(ctx, employee); err != nil {
		zapctx.Error(ctx, "Failed to create employee", zap.Error(err), zap.String("username", employee.Username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create employee"})
		return
	}

	zapctx.Info(ctx, "Employee created", zap.String("username", employee.Username))
	c.JSON(http.StatusCreated, gin.H{"status": "success", "id": employee.Username})
}

func updateEmployeeHandler(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("id")

	var employee database.EmployeeFull
	if err := c.ShouldBindJSON(&employee); err != nil {
		zapctx.Warn(ctx, "Invalid employee update request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := db.UpdateEmployee(ctx, username, employee); err != nil {
		zapctx.Error(ctx, "Failed to update employee", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update employee"})
		return
	}

	zapctx.Info(ctx, "Employee updated", zap.String("username", username))
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func deleteEmployeeHandler(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("id")

	if err := db.DeleteEmployee(ctx, username); err != nil {
		zapctx.Error(ctx, "Failed to delete employee", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete employee"})
		return
	}

	zapctx.Info(ctx, "Employee deleted", zap.String("username", username))
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// ========== Dashboard Handlers ==========

func getDashboardStatsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	// Use cache to get stats
	stats, err := dashCache.Get(ctx, db)
	if err != nil {
		zapctx.Error(ctx, "Failed to get dashboard stats", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func getActiveNowHandler(c *gin.Context) {
	ctx := c.Request.Context()

	agents, err := db.GetAgents(ctx)
	if err != nil {
		zapctx.Error(ctx, "Failed to get active agents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get active agents"})
		return
	}

	// Filter only online agents
	activeAgents := make([]database.Agent, 0)
	for _, agent := range agents {
		if agent.Status == "online" {
			activeAgents = append(activeAgents, agent)
		}
	}

	c.JSON(http.StatusOK, activeAgents)
}

// ========== Reports Handlers ==========

func getDailyReportHandler(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("username")
	dateStr := c.Query("date") // YYYY-MM-DD

	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	// Parse date or use today
	var date time.Time
	var err error
	if dateStr == "" {
		date = time.Now().In(appLocation)
	} else {
		// Parse date in app timezone
		date, err = time.ParseInLocation("2006-01-02", dateStr, appLocation)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, use YYYY-MM-DD"})
			return
		}
	}

	report, err := db.GetDailyReport(ctx, username, date)
	if err != nil {
		zapctx.Error(ctx, "Failed to get daily report", zap.Error(err), zap.String("username", username), zap.String("date", dateStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate report"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func getApplicationsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("username")
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format"})
		return
	}

	apps, err := db.GetApplicationUsage(ctx, username, start, end)
	if err != nil {
		zapctx.Error(ctx, "Failed to get applications", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get applications"})
		return
	}

	c.JSON(http.StatusOK, apps)
}

func getKeyboardEventsHandler2(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("username")
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format"})
		return
	}

	events, err := db.GetKeyboardEventsByUsername(ctx, username, start, end)
	if err != nil {
		zapctx.Error(ctx, "Failed to get keyboard events", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get keyboard events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

func getUSBEventsHandler2(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("username")
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format"})
		return
	}

	events, err := db.GetUSBEventsByUsername(ctx, username, start, end)
	if err != nil {
		zapctx.Error(ctx, "Failed to get USB events", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get USB events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

func getFileEventsHandler2(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("username")
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format"})
		return
	}

	events, err := db.GetFileEventsByUsername(ctx, username, start, end)
	if err != nil {
		zapctx.Error(ctx, "Failed to get file events", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

func getScreenshotsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	username := c.Param("username")
	startStr := c.Query("start_time")
	endStr := c.Query("end_time")

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_time format"})
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_time format"})
		return
	}

	screenshots, err := db.GetScreenshotsByUsername(ctx, username, start, end)
	if err != nil {
		zapctx.Error(ctx, "Failed to get screenshots", zap.Error(err), zap.String("username", username))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get screenshots"})
		return
	}

	c.JSON(http.StatusOK, screenshots)
}

// ========== Alerts Handlers ==========

func getAlertsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	resolvedStr := c.Query("resolved")
	severity := c.Query("severity")
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "50")

	page, _ := strconv.Atoi(pageStr)
	pageSize, _ := strconv.Atoi(pageSizeStr)

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 50
	}

	offset := (page - 1) * pageSize

	var resolved *bool
	if resolvedStr != "" {
		r := resolvedStr == "true"
		resolved = &r
	}

	alerts, err := db.GetAlerts(ctx, resolved, severity, pageSize, offset)
	if err != nil {
		zapctx.Error(ctx, "Failed to get alerts", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get alerts"})
		return
	}

	c.JSON(http.StatusOK, alerts)
}

func getUnresolvedAlertsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	falseVal := false
	alerts, err := db.GetAlerts(ctx, &falseVal, "", 100, 0)
	if err != nil {
		zapctx.Error(ctx, "Failed to get unresolved alerts", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get alerts"})
		return
	}

	c.JSON(http.StatusOK, alerts)
}

func resolveAlertHandler(c *gin.Context) {
	ctx := c.Request.Context()
	alertID := c.Param("id")

	var req struct {
		ResolvedBy string `json:"resolved_by"`
		Notes      string `json:"notes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		zapctx.Warn(ctx, "Invalid resolve alert request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := db.ResolveAlert(ctx, alertID, req.ResolvedBy); err != nil {
		zapctx.Error(ctx, "Failed to resolve alert", zap.Error(err), zap.String("alert_id", alertID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve alert"})
		return
	}

	zapctx.Info(ctx, "Alert resolved", zap.String("alert_id", alertID), zap.String("resolved_by", req.ResolvedBy))
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// getUsersListHandler returns unique list of usernames for frontend
func getUsersListHandler(c *gin.Context) {
	ctx := c.Request.Context()

	users, err := db.GetUniqueUsernames(ctx)
	if err != nil {
		zapctx.Error(ctx, "Failed to get users list", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// getActivitySegmentsHandler returns activity segments for timeline visualization
func getActivitySegmentsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	computerName := c.Query("computer_name")
	if computerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name parameter is required"})
		return
	}

	dateStr := c.Query("date")
	var date time.Time
	var err error

	if dateStr == "" {
		date = time.Now()
	} else {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			zapctx.Warn(ctx, "Invalid date format", zap.Error(err), zap.String("date", dateStr))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
			return
		}
	}

	segments, err := db.GetActivitySegments(ctx, computerName, date)
	if err != nil {
		zapctx.Error(ctx, "Failed to get activity segments", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get activity segments"})
		return
	}

	c.JSON(http.StatusOK, segments)
}
