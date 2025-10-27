package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ctolnik/Office-Monitor/server/config"
	"github.com/ctolnik/Office-Monitor/server/database"
	"github.com/ctolnik/Office-Monitor/server/storage"
	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	db  *database.Database
	st  *storage.Storage
	cfg *config.Config
)

func main() {
	var err error

	// Инициализация логгера на основе конфига
	logger := initLogger("dev") // TODO: брать из config после загрузки
	defer logger.Sync()

	cfg, err = config.Load("config.yaml")
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}
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

// initLogger создает логгер на основе окружения
func initLogger(mode string) *zap.Logger {
	var logger *zap.Logger
	var err error

	if mode == "prod" || mode == "release" {
		// Production логгер: JSON формат, без stacktrace для Info/Warn
		config := zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		logger, err = config.Build()
	} else {
		// Development логгер: удобный для чтения формат
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		logger, err = config.Build()
	}

	if err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	return logger
}

func initGin(c *config.Config, logger *zap.Logger) *gin.Engine {
	if c.Server.Mode == "prod" {
		logger.Debug("Server mode is prode")
		gin.SetMode(gin.ReleaseMode)
	}
	router := newGin(logger)
	router.LoadHTMLGlob("../web/templates/*")
	router.Static("/static", "../web/static")

	router.GET("/", indexHandler)

	api := router.Group("/api")
	{
		api.POST("/activity", receiveActivityHandler)
		api.GET("/employees", getEmployeesHandler)
		api.GET("/activity/recent", getRecentActivityHandler)

		api.POST("/usb/event", receiveUSBEventHandler)
		api.GET("/usb/events", getUSBEventsHandler)

		api.POST("/file/event", receiveFileEventHandler)
		api.GET("/file/events", getFileEventsHandler)

		api.POST("/screenshot", receiveScreenshotHandler)

		api.POST("/keyboard/event", receiveKeyboardEventHandler)
		api.GET("/keyboard/events", getKeyboardEventsHandler)
	}
	return router
}

func newGin(logger *zap.Logger) *gin.Engine {
	router := gin.New()

	// HTTP логирование
	router.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(logger, true))

	// Добавляем Request ID и логгер в контекст
	router.Use(requestIDMiddleware())
	router.Use(loggerMiddleware(logger))

	return router
}

// requestIDMiddleware добавляет уникальный ID для каждого запроса
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// loggerMiddleware добавляет логгер с request_id в контекст
func loggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, _ := c.Get("request_id")
		// Создаем логгер с request_id для трейсинга
		loggerWithReqID := logger.With(
			zap.String("request_id", requestID.(string)),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
		)
		ctx := zapctx.WithLogger(c.Request.Context(), loggerWithReqID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func receiveActivityHandler(c *gin.Context) {
	ctx := c.Request.Context()
	var event database.ActivityEvent

	if err := c.ShouldBindJSON(&event); err != nil {
		zapctx.Warn(ctx, "Invalid activity request",
			zap.Error(err),
			zap.String("remote_addr", c.ClientIP()),
		)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	zapctx.Debug(ctx, "Inserting activity event",
		zap.String("computer_name", event.ComputerName),
		zap.String("username", event.Username),
		zap.String("process", event.ProcessName),
	)

	if err := db.InsertActivityEvent(ctx, event); err != nil {
		zapctx.Error(ctx, "Failed to insert activity event",
			zap.Error(err),
			zap.String("computer_name", event.ComputerName),
			zap.String("username", event.Username),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
		return
	}

	zapctx.Info(ctx, "Activity event saved successfully",
		zap.String("computer_name", event.ComputerName),
		zap.String("username", event.Username),
	)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getEmployeesHandler(c *gin.Context) {
	ctx := c.Request.Context()

	zapctx.Debug(ctx, "Fetching active employees")

	employees, err := db.GetActiveEmployees(ctx)
	if err != nil {
		zapctx.Error(ctx, "Failed to fetch employees", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employees"})
		return
	}

	zapctx.Info(ctx, "Employees fetched successfully",
		zap.Int("count", len(employees)),
	)
	c.JSON(http.StatusOK, employees)
}

func getRecentActivityHandler(c *gin.Context) {
	ctx := c.Request.Context()

	zapctx.Debug(ctx, "Fetching recent activity")

	records, err := db.GetRecentActivity(ctx, 100)
	if err != nil {
		zapctx.Error(ctx, "Failed to fetch recent activity", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity"})
		return
	}

	zapctx.Info(ctx, "Recent activity fetched successfully",
		zap.Int("count", len(records)),
	)
	c.JSON(http.StatusOK, records)
}

func receiveUSBEventHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var event database.USBEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if err := db.InsertUSBEvent(ctx, event); err != nil {
		zapctx.Error(ctx, "Failed to insert USB event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getUSBEventsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	computerName := c.Query("computer_name")
	if computerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
		zapctx.Warn(ctx, "Computer_name is empty")
		return
	}

	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

	from, _ := time.Parse(time.RFC3339, fromStr)
	to, _ := time.Parse(time.RFC3339, toStr)

	events, err := db.GetUSBEvents(ctx, computerName, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

func receiveFileEventHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var event database.FileCopyEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if err := db.InsertFileCopyEvent(ctx, event); err != nil {
		zapctx.Error(ctx, "Failed to insert file event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getFileEventsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	computerName := c.Query("computer_name")
	if computerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
		return
	}

	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

	from, _ := time.Parse(time.RFC3339, fromStr)
	to, _ := time.Parse(time.RFC3339, toStr)

	events, err := db.GetFileEvents(ctx, computerName, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

func receiveScreenshotHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var screenshot struct {
		Timestamp    time.Time `json:"timestamp"`
		ComputerName string    `json:"computer_name"`
		Username     string    `json:"username"`
		ScreenshotID string    `json:"screenshot_id"`
		WindowTitle  string    `json:"window_title"`
		ProcessName  string    `json:"process_name"`
		FileSize     int64     `json:"file_size"`
		ImageData    []byte    `json:"image_data"`
	}

	if err := c.ShouldBindJSON(&screenshot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if screenshot.Timestamp.IsZero() {
		screenshot.Timestamp = time.Now()
	}

	minioPath, err := st.UploadScreenshot(ctx, screenshot.ScreenshotID, screenshot.ImageData)
	if err != nil {
		zapctx.Error(ctx, "Failed to upload screenshot to MinIO", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save screenshot"})
		return
	}

	meta := database.ScreenshotMetadata{
		Timestamp:    screenshot.Timestamp,
		ComputerName: screenshot.ComputerName,
		Username:     screenshot.Username,
		ScreenshotID: screenshot.ScreenshotID,
		MinIOPath:    minioPath,
		FileSize:     uint64(screenshot.FileSize),
		WindowTitle:  screenshot.WindowTitle,
		ProcessName:  screenshot.ProcessName,
	}

	if err := db.InsertScreenshotMetadata(ctx, meta); err != nil {
		zapctx.Error(ctx, "Failed to insert screenshot metadata", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metadata"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "screenshot_id": screenshot.ScreenshotID})
}

func receiveKeyboardEventHandler(c *gin.Context) {
	ctx := c.Request.Context()

	var event database.KeyboardEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	if err := db.InsertKeyboardEvent(ctx, event); err != nil {
		zapctx.Error(ctx, "Failed to insert keyboard event", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getKeyboardEventsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	computerName := c.Query("computer_name")
	if computerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
		return
	}

	fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

	from, _ := time.Parse(time.RFC3339, fromStr)
	to, _ := time.Parse(time.RFC3339, toStr)

	events, err := db.GetKeyboardEvents(ctx, computerName, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	c.JSON(http.StatusOK, events)
}
