package ipc

import "time"

type EventType string

const (
	EventTypeActivity EventType = "activity_segment"
	EventTypeKeyboard EventType = "keyboard_event"
	EventTypeHeartbeat EventType = "heartbeat"
)

type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	SessionID string      `json:"session_id"`
	Username  string      `json:"username"`
	Data      interface{} `json:"data"`
}

type ActivitySegment struct {
	TimestampStart time.Time `json:"timestamp_start"`
	TimestampEnd   time.Time `json:"timestamp_end"`
	DurationSec    uint32    `json:"duration_sec"`
	State          string    `json:"state"`
	ProcessName    string    `json:"process_name"`
	WindowTitle    string    `json:"window_title"`
	Category       string    `json:"category,omitempty"`
}

type KeyboardEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	ProcessName string    `json:"process_name"`
	WindowTitle string    `json:"window_title"`
	CharCount   int       `json:"char_count"`
	Content     string    `json:"content,omitempty"`
}

type Heartbeat struct {
	Uptime      int64  `json:"uptime_sec"`
	MemoryMB    int    `json:"memory_mb"`
	ProcessName string `json:"foreground_process"`
	IdleTimeSec int    `json:"idle_time_sec"`
}

const (
	PipeName = `\\.\pipe\office-monitor-events`
)
