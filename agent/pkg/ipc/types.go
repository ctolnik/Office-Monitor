package ipc

import "time"

type EventType string

const (
	EventTypeActivity   EventType = "activity_segment"
	EventTypeKeyboard   EventType = "keyboard_event"
	EventTypeHeartbeat  EventType = "heartbeat"
	EventTypeShotBegin  EventType = "screenshot_begin"
	EventTypeShotChunk  EventType = "screenshot_chunk"
	EventTypeShotCommit EventType = "screenshot_commit"
	EventTypeAck        EventType = "ack"
	EventTypeError      EventType = "error"
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

type ScreenshotBegin struct {
	ScreenshotID string    `json:"screenshot_id"`
	Timestamp    time.Time `json:"timestamp"`
	ComputerName string    `json:"computer_name"`
	Username     string    `json:"username"`
	SessionID    string    `json:"session_id"`
	ProcessName  string    `json:"process_name,omitempty"`
	WindowTitle  string    `json:"window_title,omitempty"`
	Quality      int       `json:"quality,omitempty"`
	MimeType     string    `json:"mime_type"`
	TotalSize    int       `json:"total_size"`
	SHA256       string    `json:"sha256"`
}

type ScreenshotChunk struct {
	ScreenshotID string `json:"screenshot_id"`
	Offset       int    `json:"offset"`
	Data         []byte `json:"data"` // base64 in JSON
}

type ScreenshotCommit struct {
	ScreenshotID string `json:"screenshot_id"`
}

type Ack struct {
	RefID string `json:"ref_id,omitempty"`
	Msg   string `json:"msg,omitempty"`
}

type Error struct {
	RefID string `json:"ref_id,omitempty"`
	Msg   string `json:"msg"`
}

const (
	PipeName = `\\.\pipe\office-monitor-events`
)
