package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ctolnik/Office-Monitor/server/config"
	"github.com/ctolnik/Office-Monitor/server/database"
	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
)

var (
	db *database.Database
	// st  *storage.Storage
	cfg *config.Config
)

func main() {
	var err error

	// logger, error := zap.NewProduction()
	logger, error := zap.NewDevelopment()
	if error != nil {
		log.Fatal("Failed to init logger")
	}
	defer logger.Sync()

	cfg, err = config.Load("config.yaml")
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}
	fmt.Println(cfg)
	db, err = database.New(
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.Username,
		cfg.Database.Password,
	)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()
	router := initGin(cfg, logger)

	sock := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("Server starting", zap.String("Socket", sock))
	if err := router.Run(sock); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}

func initGin(c *config.Config, logger *zap.Logger) *gin.Engine {
	if c.Server.Mode == "prod" {
		logger.Debug("Server mode is prode")
		gin.SetMode(gin.ReleaseMode)
	}
	router := newGin(logger)
	// router := gin.Default()
	router.LoadHTMLGlob("../web/templates/*")
	router.Static("/static", "../web/static")

	router.GET("/", indexHandler)

	api := router.Group("/api")
	{
		api.POST("/activity", receiveActivityHandler)
		api.GET("/employees", getEmployeesHandler)
		api.GET("/activity/recent", getRecentActivityHandler)

		// api.POST("/usb/event", receiveUSBEventHandler)
		// api.GET("/usb/events", getUSBEventsHandler)

		// api.POST("/file/event", receiveFileEventHandler)
		// api.GET("/file/events", getFileEventsHandler)

		// api.POST("/screenshot", receiveScreenshotHandler)

		// api.POST("/keyboard/event", receiveKeyboardEventHandler)
		// api.GET("/keyboard/events", getKeyboardEventsHandler)
	}
	return router
}

func newGin(logger *zap.Logger) *gin.Engine {
	router := gin.New()

	// HTTP логирование
	router.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(logger, true))

	// Добавляем логгер в контекст
	router.Use(func(c *gin.Context) {
		ctx := zapctx.WithLogger(c.Request.Context(), logger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	return router
}

func indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func receiveActivityHandler(c *gin.Context) {
	var event database.ActivityEvent

	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	ctx := context.Background()
	if err := db.InsertActivityEvent(ctx, event); err != nil {
		log.Printf("Failed to insert activity: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getEmployeesHandler(c *gin.Context) {
	ctx := context.Background()
	employees, err := db.GetActiveEmployees(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employees"})
		return
	}

	c.JSON(http.StatusOK, employees)
}

func getRecentActivityHandler(c *gin.Context) {
	ctx := context.Background()
	records, err := db.GetRecentActivity(ctx, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity"})
		return
	}

	c.JSON(http.StatusOK, records)
}

// func receiveUSBEventHandler(c *gin.Context) {
// 	var event database.USBEvent
// 	if err := c.ShouldBindJSON(&event); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
// 		return
// 	}

// 	if event.Timestamp.IsZero() {
// 		event.Timestamp = time.Now()
// 	}

// 	ctx := context.Background()
// 	if err := db.InsertUSBEvent(ctx, event); err != nil {
// 		log.Printf("Failed to insert USB event: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"status": "success"})
// }

// func getUSBEventsHandler(c *gin.Context) {
// 	computerName := c.Query("computer_name")
// 	if computerName == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
// 		return
// 	}

// 	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
// 	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

// 	from, _ := time.Parse(time.RFC3339, fromStr)
// 	to, _ := time.Parse(time.RFC3339, toStr)

// 	ctx := context.Background()
// 	events, err := db.GetUSBEvents(ctx, computerName, from, to)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, events)
// }

// func receiveFileEventHandler(c *gin.Context) {
// 	var event database.FileCopyEvent
// 	if err := c.ShouldBindJSON(&event); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
// 		return
// 	}

// 	if event.Timestamp.IsZero() {
// 		event.Timestamp = time.Now()
// 	}

// 	ctx := context.Background()
// 	if err := db.InsertFileCopyEvent(ctx, event); err != nil {
// 		log.Printf("Failed to insert file event: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"status": "success"})
// }

// func getFileEventsHandler(c *gin.Context) {
// 	computerName := c.Query("computer_name")
// 	if computerName == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
// 		return
// 	}

// 	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
// 	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

// 	from, _ := time.Parse(time.RFC3339, fromStr)
// 	to, _ := time.Parse(time.RFC3339, toStr)

// 	ctx := context.Background()
// 	events, err := db.GetFileEvents(ctx, computerName, from, to)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, events)
// }

// func receiveScreenshotHandler(c *gin.Context) {
// 	var screenshot struct {
// 		Timestamp    time.Time `json:"timestamp"`
// 		ComputerName string    `json:"computer_name"`
// 		Username     string    `json:"username"`
// 		ScreenshotID string    `json:"screenshot_id"`
// 		WindowTitle  string    `json:"window_title"`
// 		ProcessName  string    `json:"process_name"`
// 		FileSize     int64     `json:"file_size"`
// 		ImageData    []byte    `json:"image_data"`
// 	}

// 	if err := c.ShouldBindJSON(&screenshot); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
// 		return
// 	}

// 	if screenshot.Timestamp.IsZero() {
// 		screenshot.Timestamp = time.Now()
// 	}

// 	ctx := context.Background()

// 	minioPath, err := st.UploadScreenshot(ctx, screenshot.ScreenshotID, screenshot.ImageData)
// 	if err != nil {
// 		log.Printf("Failed to upload screenshot to MinIO: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save screenshot"})
// 		return
// 	}

// 	meta := database.ScreenshotMetadata{
// 		Timestamp:    screenshot.Timestamp,
// 		ComputerName: screenshot.ComputerName,
// 		Username:     screenshot.Username,
// 		ScreenshotID: screenshot.ScreenshotID,
// 		MinIOPath:    minioPath,
// 		FileSize:     uint64(screenshot.FileSize),
// 		WindowTitle:  screenshot.WindowTitle,
// 		ProcessName:  screenshot.ProcessName,
// 	}

// 	if err := db.InsertScreenshotMetadata(ctx, meta); err != nil {
// 		log.Printf("Failed to insert screenshot metadata: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metadata"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"status": "success", "screenshot_id": screenshot.ScreenshotID})
// }

// func receiveKeyboardEventHandler(c *gin.Context) {
// 	var event database.KeyboardEvent
// 	if err := c.ShouldBindJSON(&event); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
// 		return
// 	}

// 	if event.Timestamp.IsZero() {
// 		event.Timestamp = time.Now()
// 	}

// 	ctx := context.Background()
// 	if err := db.InsertKeyboardEvent(ctx, event); err != nil {
// 		log.Printf("Failed to insert keyboard event: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"status": "success"})
// }

// func getKeyboardEventsHandler(c *gin.Context) {
// 	computerName := c.Query("computer_name")
// 	if computerName == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
// 		return
// 	}

// 	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
// 	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

// 	from, _ := time.Parse(time.RFC3339, fromStr)
// 	to, _ := time.Parse(time.RFC3339, toStr)

// 	ctx := context.Background()
// 	events, err := db.GetKeyboardEvents(ctx, computerName, from, to)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, events)
// }
