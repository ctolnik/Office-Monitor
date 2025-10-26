package main

import (
	"fmt"
	"log"

	"net/http"

	"github.com/ctolnik/Office-Monitor/server/config"
	"go.uber.org/zap"

	// "employee-monitor/server/config"
	// "employee-monitor/server/database"
	// "employee-monitor/server/storage"

	"github.com/gin-gonic/gin"
)

var (
	// db  *database.Database
	// st  *storage.Storage
	cfg *config.Config
)

func main() {
	var err error

	logger, error := zap.NewDevelopment()
	if error != nil {
		log.Fatal("Failed to init logger")
	}
	defer logger.Sync()

	cfg, err = config.Load("config.yaml")
	if err != nil {
		logger.Fatal("Failed to load config:", zap.Error(err))
	}

	// db, err = database.New(
	// 	cfg.Database.ClickHouse.Host,
	// 	cfg.Database.ClickHouse.Port,
	// 	cfg.Database.ClickHouse.Database,
	// 	cfg.Database.ClickHouse.Username,
	// 	cfg.Database.ClickHouse.Password,
	// )
	// if err != nil {
	// 	log.Fatalf("Failed to connect to database: %v", err)
	// }
	// defer db.Close()

	// st, err = storage.New(
	// 	cfg.Storage.MinIO.Endpoint,
	// 	cfg.Storage.MinIO.AccessKey,
	// 	cfg.Storage.MinIO.SecretKey,
	// 	cfg.Storage.MinIO.UseSSL,
	// )
	// if err != nil {
	// 	log.Fatalf("Failed to connect to MinIO: %v", err)
	// }

	if cfg.Server.Mode == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.LoadHTMLGlob("../web/templates/*")
	router.Static("/static", "../web/static")

	router.GET("/", indexHandler)

	// api := router.Group("/api")
	// {
	// 	api.POST("/activity", receiveActivityHandler)
	// 	api.GET("/employees", getEmployeesHandler)
	// 	api.GET("/activity/recent", getRecentActivityHandler)

	// 	api.POST("/usb/event", receiveUSBEventHandler)
	// 	api.GET("/usb/events", getUSBEventsHandler)

	// 	api.POST("/file/event", receiveFileEventHandler)
	// 	api.GET("/file/events", getFileEventsHandler)

	// 	api.POST("/screenshot", receiveScreenshotHandler)

	// 	api.POST("/keyboard/event", receiveKeyboardEventHandler)
	// 	api.GET("/keyboard/events", getKeyboardEventsHandler)
	// }

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Info("Server starting on: ", zap.String("address", addr))
	// log.Printf("Server starting on %s", addr)
	if err := router.Run(addr); err != nil {
		logger.Fatal("Failed to start server:", zap.Error(err))
		// log.Fatalf("Failed to start server: %v", err)
	}
}

func indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

// func receiveActivityHandler(c *gin.Context) {
// 	var event database.ActivityEvent
// 	if err := c.ShouldBindJSON(&event); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
// 		return
// 	}

// 	if event.Timestamp.IsZero() {
// 		event.Timestamp = time.Now()
// 	}

// 	ctx := context.Background()
// 	if err := db.InsertActivityEvent(ctx, event); err != nil {
// 		log.Printf("Failed to insert activity: %v", err)
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"status": "success"})
// }

// func getEmployeesHandler(c *gin.Context) {
// 	ctx := context.Background()
// 	employees, err := db.GetActiveEmployees(ctx)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch employees"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, employees)
// }

// func getRecentActivityHandler(c *gin.Context) {
// 	ctx := context.Background()
// 	records, err := db.GetRecentActivity(ctx, 100)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activity"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, records)
// }

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
