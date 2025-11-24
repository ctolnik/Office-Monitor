package database

import "time"

type ActivityEvent struct {
        Timestamp    time.Time `json:"timestamp"`
        ComputerName string    `json:"computer_name"`
        Username     string    `json:"username"`
        WindowTitle  string    `json:"window_title"`
        ProcessName  string    `json:"process_name"`
        Duration     uint32    `json:"duration"`
}

type KeyboardEvent struct {
        Timestamp    time.Time `json:"timestamp"`
        ComputerName string    `json:"computer_name"`
        Username     string    `json:"username"`
        WindowTitle  string    `json:"window_title"`
        ProcessName  string    `json:"process_name"`
        TextContent  string    `json:"text_content"`
}

type ActivitySegment struct {
        TimestampStart time.Time `json:"timestamp_start"`
        TimestampEnd   time.Time `json:"timestamp_end"`
        DurationSec    uint32    `json:"duration_sec"`
        State          string    `json:"state"`
        ComputerName   string    `json:"computer_name"`
        Username       string    `json:"username"`
        ProcessName    string    `json:"process_name"`
        WindowTitle    string    `json:"window_title"`
        SessionID      string    `json:"session_id"`
}

type ProcessCatalogEntry struct {
        ID                   string    `json:"id"`
        FriendlyName         string    `json:"friendly_name"`
        ProcessNames         []string  `json:"process_names"`
        WindowTitlePatterns  []string  `json:"window_title_patterns"`
        Category             string    `json:"category"`
        IsActive             bool      `json:"is_active"`
        CreatedAt            time.Time `json:"created_at"`
        UpdatedAt            time.Time `json:"updated_at"`
}

type DailyActivitySummary struct {
        Date              string                   `json:"date"`
        ComputerName      string                   `json:"computer_name"`
        Username          string                   `json:"username"`
        ActiveSeconds     uint64                   `json:"active_seconds"`
        IdleSeconds       uint64                   `json:"idle_seconds"`
        OfflineSeconds    uint64                   `json:"offline_seconds"`
        TopPrograms       []ProgramUsage           `json:"top_programs"`
}

type ProgramUsage struct {
        ProcessName   string `json:"process_name"`
        FriendlyName  string `json:"friendly_name"`
        DurationSec   uint64 `json:"duration_sec"`
        WindowTitles  []string `json:"window_titles,omitempty"`
}

type FileCopyEvent struct {
        Timestamp       time.Time `json:"timestamp"`
        ComputerName    string    `json:"computer_name"`
        Username        string    `json:"username"`
        SourcePath      string    `json:"source_path"`
        DestinationPath string    `json:"destination_path"`
        FileSize        uint64    `json:"file_size"`
        FileCount       uint32    `json:"file_count"`
        OperationType   string    `json:"operation_type"`
        IsUSBTarget     uint8     `json:"is_usb_target"`
}

type USBEvent struct {
        Timestamp    time.Time `json:"timestamp"`
        ComputerName string    `json:"computer_name"`
        Username     string    `json:"username"`
        DeviceID     string    `json:"device_id"`
        DeviceName   string    `json:"device_name"`
        DeviceType   string    `json:"device_type"`
        EventType    string    `json:"event_type"`
        VolumeSerial string    `json:"volume_serial"`
}

type ScreenshotMetadata struct {
        Timestamp    time.Time `json:"timestamp"`
        ComputerName string    `json:"computer_name"`
        Username     string    `json:"username"`
        ScreenshotID string    `json:"screenshot_id"`
        MinIOPath    string    `json:"minio_path"`
        FileSize     uint64    `json:"file_size"`
        WindowTitle  string    `json:"window_title"`
        ProcessName  string    `json:"process_name"`
}

type Alert struct {
        Timestamp       time.Time `json:"timestamp"`
        ComputerName    string    `json:"computer_name"`
        Username        string    `json:"username"`
        AlertType       string    `json:"alert_type"`
        Severity        string    `json:"severity"`
        Description     string    `json:"description"`
        Metadata        string    `json:"metadata"`
        IsAcknowledged  bool      `json:"is_acknowledged"`
}

type Employee struct {
        ComputerName string    `json:"computer_name"`
        Username     string    `json:"username"`
        LastSeen     time.Time `json:"last_seen"`
        Status       string    `json:"status"`
}

type AgentConfig struct {
        ComputerName                 string    `json:"computer_name"`
        APIKey                       string    `json:"api_key"`
        ScreenshotEnabled            bool      `json:"screenshot_enabled"`
        ScreenshotIntervalMinutes    int       `json:"screenshot_interval_minutes"`
        KeyloggerEnabled             bool      `json:"keylogger_enabled"`
        USBMonitoringEnabled         bool      `json:"usb_monitoring_enabled"`
        FileCopyMonitoringEnabled    bool      `json:"file_copy_monitoring_enabled"`
        LargeCopyThresholdMB         int       `json:"large_copy_threshold_mb"`
        LastSeen                     time.Time `json:"last_seen"`
        AgentVersion                 string    `json:"agent_version"`
}

type Agent struct {
        ComputerName string       `json:"computer_name"`
        Username     string       `json:"username"`
        LastSeen     string       `json:"last_seen"`
        Status       string       `json:"status"`
        IPAddress    string       `json:"ip_address"`
        OSVersion    string       `json:"os_version"`
        AgentVersion string       `json:"agent_version"`
        Config       ConfigUpdate `json:"config"`
}

type ConfigUpdate struct {
        ScreenshotInterval int  `json:"screenshot_interval"`
        ActivityTracking   bool `json:"activity_tracking"`
        KeyloggerEnabled   bool `json:"keylogger_enabled"`
        USBMonitoring      bool `json:"usb_monitoring"`
        FileMonitoring     bool `json:"file_monitoring"`
        DLPEnabled         bool `json:"dlp_enabled"`
}

type EmployeeFull struct {
        ID                 string   `json:"id"`
        ComputerName       string   `json:"computer_name"`
        Username           string   `json:"username"`
        FullName           string   `json:"full_name"`
        Email              string   `json:"email"`
        Department         string   `json:"department"`
        Position           string   `json:"position"`
        LastSeen           time.Time `json:"last_seen"`
        Status             string   `json:"status"`
        ActiveTime         uint64   `json:"active_time"`
        IdleTime           uint64   `json:"idle_time"`
        ProductivityScore  float64  `json:"productivity_score"`
        TopApplications    []string `json:"top_applications"`
        RecentScreenshots  []string `json:"recent_screenshots"`
        ConsentGiven       bool     `json:"consent_given"`
        ConsentDate        *string  `json:"consent_date"`
        IsActive           bool     `json:"is_active"`
        CreatedAt          string   `json:"created_at"`
}

type ApplicationCategory struct {
        ID             string    `json:"id"`
        ProcessName    string    `json:"process_name"`
        ProcessPattern string    `json:"process_pattern"`
        Category       string    `json:"category"`
        CreatedAt      time.Time `json:"created_at"`
        UpdatedAt      time.Time `json:"updated_at"`
        CreatedBy      string    `json:"created_by"`
        UpdatedBy      string    `json:"updated_by"`
        IsActive       bool      `json:"is_active"`
}

type KeyboardPeriod struct {
        Start          string `json:"start"`
        End            string `json:"end"`
        ComputerName   string `json:"computer_name"`
        Username       string `json:"username"`
        Application    string `json:"application"`
        ProcessName    string `json:"process_name"`
        WindowTitle    string `json:"window_title"`
        FormattedText  string `json:"formatted_text"`
        RawKeys        string `json:"raw_keys"`
        CharacterCount int    `json:"character_count"`
}

type DashboardStats struct {
        TotalAgents       int     `json:"total_agents"`
        OnlineAgents      int     `json:"online_agents"`
        IdleAgents        int     `json:"idle_agents"`
        OfflineAgents     int     `json:"offline_agents"`
        TotalAlerts       int     `json:"total_alerts"`
        CriticalAlerts    int     `json:"critical_alerts"`
        TotalScreenshots  int     `json:"total_screenshots"`
        AverageActivity   float64 `json:"average_activity"`
        TotalEmployees    int     `json:"total_employees"`
        ActiveNow         int     `json:"active_now"`
        Offline           int     `json:"offline"`
        UnresolvedAlerts  int     `json:"unresolved_alerts"`
        TodayScreenshots  int     `json:"today_screenshots"`
        TodayUSBEvents    int     `json:"today_usb_events"`
        TodayFileEvents   int     `json:"today_file_events"`
}

type ApplicationUsage struct {
        ApplicationName string  `json:"application_name"`
        Category        string  `json:"category"`
        TotalDuration   uint64  `json:"total_duration"`
        SessionCount    int     `json:"session_count"`
        UniqueUsers     int     `json:"unique_users"`
        ProductiveTime  uint64  `json:"productive_time"`
        UnproductiveTime uint64 `json:"unproductive_time"`
}

type AlertFull struct {
        ID             string    `json:"id"`
        Timestamp      time.Time `json:"timestamp"`
        ComputerName   string    `json:"computer_name"`
        Username       string    `json:"username"`
        AlertType      string    `json:"alert_type"`
        Severity       string    `json:"severity"`
        Description    string    `json:"description"`
        Metadata       string    `json:"metadata"`
        IsAcknowledged bool      `json:"is_acknowledged"`
        AcknowledgedBy string    `json:"acknowledged_by"`
        AcknowledgedAt *time.Time `json:"acknowledged_at"`
}

type DailyReport struct {
        Date              string                   `json:"date"`
        ComputerName      string                   `json:"computer_name"`
        Username          string                   `json:"username"`
        TotalActiveTime   uint64                   `json:"total_active_time"`
        TotalIdleTime     uint64                   `json:"total_idle_time"`
        ProductivityScore float64                  `json:"productivity_score"`
        Applications      []ApplicationUsage       `json:"applications"`
        Screenshots       []ScreenshotMetadata     `json:"screenshots"`
        KeyboardActivity  []KeyboardPeriod         `json:"keyboard_activity"`
        USBEvents         []USBEvent               `json:"usb_events"`
        Alerts            []AlertFull              `json:"alerts"`
}
