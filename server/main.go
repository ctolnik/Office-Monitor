package main

import (
        "context"
        "fmt"
        "log"
        "net/http"
        "time"

        "github.com/ctolnik/Office-Monitor/server/config"
        "github.com/ctolnik/Office-Monitor/server/database"
        "github.com/ctolnik/Office-Monitor/server/storage"

        "github.com/gin-gonic/gin"
)

var (
        db            *database.Database
        st            *storage.Storage
        cfg           *config.Config
        storageClient *storage.Storage
        appLocation   *time.Location
        dashCache     *DashboardCache
)

func main() {
        var err error

        cfg, err = config.Load("config.yaml")
        if err != nil {
                log.Fatalf("Failed to load config: %v", err)
        }
        
        // Initialize timezone
        appLocation, err = time.LoadLocation(cfg.Database.Timezone)
        if err != nil {
                log.Printf("Failed to load timezone %s, using UTC: %v", cfg.Database.Timezone, err)
                appLocation = time.UTC
        }
        
        // Initialize cache with 30 second TTL
        dashCache = NewDashboardCache(30 * time.Second)

        db, err = database.New(
                cfg.Database.Host,
                cfg.Database.Port,
                cfg.Database.Database,
                cfg.Database.Username,
                cfg.Database.Password,
        )
        if err != nil {
                log.Fatalf("Failed to connect to database: %v", err)
        }
        defer db.Close()

        st, err = storage.New(
                cfg.Storage.Endpoint,
                cfg.Storage.AccessKey,
                cfg.Storage.SecretKey,
                cfg.Storage.UseSSL,
                cfg.Storage.Buckets.Screenshots,
                cfg.Storage.Buckets.USBCopies,
                cfg.Storage.PublicEndpoint,
        )
        if err != nil {
                log.Fatalf("Failed to connect to MinIO: %v", err)
        }
        storageClient = st

        if cfg.Server.Mode == "release" {
                gin.SetMode(gin.ReleaseMode)
        }

        router := gin.Default()
        router.LoadHTMLGlob("web/templates/*")
        router.Static("/static", "web/static")

        router.GET("/", indexHandler)

        api := router.Group("/api")
        {
                api.POST("/activity", receiveActivityHandler)
                api.GET("/employees", getEmployeesHandler)
                api.GET("/activity/recent", getRecentActivityHandler)

                api.POST("/activity/segment", receiveActivitySegmentHandler)
                api.GET("/activity/summary", getDailyActivitySummaryHandler)

                api.POST("/usb/event", receiveUSBEventHandler)
                api.GET("/usb/events", getUSBEventsHandler)

                api.POST("/file/event", receiveFileEventHandler)
                api.GET("/file/events", getFileEventsHandler)

                api.POST("/screenshot", receiveScreenshotHandler)

                api.POST("/keyboard/event", receiveKeyboardEventHandler)
                api.GET("/keyboard/events", getKeyboardEventsHandler)

                api.GET("/process-catalog", getProcessCatalogHandler)
                api.POST("/process-catalog", createProcessCatalogHandler)
                api.PUT("/process-catalog/:id", updateProcessCatalogHandler)
                api.DELETE("/process-catalog/:id", deleteProcessCatalogHandler)
        }

        addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
        log.Printf("Server starting on %s", addr)
        if err := router.Run(addr); err != nil {
                log.Fatalf("Failed to start server: %v", err)
        }
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

func receiveUSBEventHandler(c *gin.Context) {
        var event database.USBEvent
        if err := c.ShouldBindJSON(&event); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
                return
        }

        if event.Timestamp.IsZero() {
                event.Timestamp = time.Now()
        }

        ctx := context.Background()
        if err := db.InsertUSBEvent(ctx, event); err != nil {
                log.Printf("Failed to insert USB event: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
                return
        }

        c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getUSBEventsHandler(c *gin.Context) {
        computerName := c.Query("computer_name")
        if computerName == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
                return
        }

        fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
        toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

        from, _ := time.Parse(time.RFC3339, fromStr)
        to, _ := time.Parse(time.RFC3339, toStr)

        ctx := context.Background()
        events, err := db.GetUSBEvents(ctx, computerName, from, to)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
                return
        }

        c.JSON(http.StatusOK, events)
}

func receiveFileEventHandler(c *gin.Context) {
        var event database.FileCopyEvent
        if err := c.ShouldBindJSON(&event); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
                return
        }

        if event.Timestamp.IsZero() {
                event.Timestamp = time.Now()
        }

        ctx := context.Background()
        if err := db.InsertFileCopyEvent(ctx, event); err != nil {
                log.Printf("Failed to insert file event: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
                return
        }

        c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getFileEventsHandler(c *gin.Context) {
        computerName := c.Query("computer_name")
        if computerName == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
                return
        }

        fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
        toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

        from, _ := time.Parse(time.RFC3339, fromStr)
        to, _ := time.Parse(time.RFC3339, toStr)

        ctx := context.Background()
        events, err := db.GetFileEvents(ctx, computerName, from, to)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
                return
        }

        c.JSON(http.StatusOK, events)
}

func receiveScreenshotHandler(c *gin.Context) {
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

        ctx := context.Background()

        minioPath, err := st.UploadScreenshot(ctx, screenshot.ScreenshotID, screenshot.ImageData)
        if err != nil {
                log.Printf("Failed to upload screenshot to MinIO: %v", err)
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
                log.Printf("Failed to insert screenshot metadata: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metadata"})
                return
        }

        c.JSON(http.StatusOK, gin.H{"status": "success", "screenshot_id": screenshot.ScreenshotID})
}

func receiveKeyboardEventHandler(c *gin.Context) {
        var event database.KeyboardEvent
        if err := c.ShouldBindJSON(&event); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
                return
        }

        if event.Timestamp.IsZero() {
                event.Timestamp = time.Now()
        }

        ctx := context.Background()
        if err := db.InsertKeyboardEvent(ctx, event); err != nil {
                log.Printf("Failed to insert keyboard event: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
                return
        }

        c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getKeyboardEventsHandler(c *gin.Context) {
        computerName := c.Query("computer_name")
        if computerName == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
                return
        }

        fromStr := c.DefaultQuery("from", time.Now().AddDate(0, 0, -7).Format(time.RFC3339))
        toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

        from, _ := time.Parse(time.RFC3339, fromStr)
        to, _ := time.Parse(time.RFC3339, toStr)

        ctx := context.Background()
        events, err := db.GetKeyboardEvents(ctx, computerName, from, to)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
                return
        }

        c.JSON(http.StatusOK, events)
}

func receiveActivitySegmentHandler(c *gin.Context) {
        var segment database.ActivitySegment
        if err := c.ShouldBindJSON(&segment); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
                return
        }

        if segment.TimestampStart.IsZero() {
                segment.TimestampStart = time.Now()
        }
        if segment.TimestampEnd.IsZero() {
                segment.TimestampEnd = segment.TimestampStart
        }

        ctx := context.Background()
        if err := db.InsertActivitySegment(ctx, segment); err != nil {
                log.Printf("Failed to insert activity segment: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save"})
                return
        }

        c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func getDailyActivitySummaryHandler(c *gin.Context) {
        computerName := c.Query("computer_name")
        if computerName == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "computer_name required"})
                return
        }

        dateStr := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
        date, err := time.Parse("2006-01-02", dateStr)
        if err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
                return
        }

        ctx := context.Background()
        summary, err := db.GetDailyActivitySummary(ctx, computerName, date)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch summary"})
                return
        }

        c.JSON(http.StatusOK, summary)
}

func getProcessCatalogHandler(c *gin.Context) {
        ctx := context.Background()
        entries, err := db.GetProcessCatalog(ctx)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch catalog"})
                return
        }

        c.JSON(http.StatusOK, entries)
}

func createProcessCatalogHandler(c *gin.Context) {
        var entry database.ProcessCatalogEntry
        if err := c.ShouldBindJSON(&entry); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
                return
        }

        entry.ID = fmt.Sprintf("%d", time.Now().UnixNano())
        entry.CreatedAt = time.Now()
        entry.UpdatedAt = time.Now()
        entry.IsActive = true

        ctx := context.Background()
        if err := db.CreateProcessCatalogEntry(ctx, entry); err != nil {
                log.Printf("Failed to create process catalog entry: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create"})
                return
        }

        c.JSON(http.StatusOK, entry)
}

func updateProcessCatalogHandler(c *gin.Context) {
        id := c.Param("id")
        if id == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "ID required"})
                return
        }

        var entry database.ProcessCatalogEntry
        if err := c.ShouldBindJSON(&entry); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
                return
        }

        entry.ID = id
        entry.UpdatedAt = time.Now()

        ctx := context.Background()
        if err := db.UpdateProcessCatalogEntry(ctx, entry); err != nil {
                log.Printf("Failed to update process catalog entry: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update"})
                return
        }

        c.JSON(http.StatusOK, entry)
}

func deleteProcessCatalogHandler(c *gin.Context) {
        id := c.Param("id")
        if id == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "ID required"})
                return
        }

        ctx := context.Background()
        if err := db.DeleteProcessCatalogEntry(ctx, id); err != nil {
                log.Printf("Failed to delete process catalog entry: %v", err)
                c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete"})
                return
        }

        c.JSON(http.StatusOK, gin.H{"status": "success"})
}
