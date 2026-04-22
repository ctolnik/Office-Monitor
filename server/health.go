package main

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// healthHandler reports the real readiness of the backend and its external
// dependencies. It is cheap enough to be polled by nginx, docker-compose and
// the agent at short intervals. The handler intentionally returns HTTP 503
// when any hard dependency is degraded so upstream probes can react.
func healthHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	dbStatus := "ok"
	switch {
	case db == nil:
		dbStatus = "not_initialized"
	default:
		if err := db.Ping(ctx); err != nil {
			dbStatus = "error: " + err.Error()
		}
	}

	storageStatus := "ok"
	switch {
	case storageClient == nil:
		storageStatus = "not_configured"
	default:
		if err := storageClient.Ping(ctx); err != nil {
			storageStatus = "error: " + err.Error()
		}
	}

	overallOK := dbStatus == "ok" && (storageStatus == "ok" || storageStatus == "not_configured")
	status := http.StatusOK
	overall := "ok"
	if !overallOK {
		status = http.StatusServiceUnavailable
		overall = "degraded"
	}

	uptime := time.Duration(0)
	if !startTime.IsZero() {
		uptime = time.Since(startTime)
	}

	c.JSON(status, gin.H{
		"status":         overall,
		"database":       dbStatus,
		"storage":        storageStatus,
		"uptime_seconds": int64(uptime.Seconds()),
	})
}
