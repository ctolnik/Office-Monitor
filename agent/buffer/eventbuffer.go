package buffer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ctolnik/Office-Monitor/agent/httpclient"
)

const (
	defaultBufferSize  = 1000
	defaultFlushSize   = 50
	defaultFlushPeriod = 30 * time.Second
	maxRetries         = 3
)

// Event represents a generic buffered event
type Event struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// EventBuffer buffers events and flushes them to server
type EventBuffer struct {
	client       *httpclient.Client
	endpoint     string
	buffer       []Event
	bufferFile   string
	maxSize      int
	flushSize    int
	flushPeriod  time.Duration
	mu           sync.Mutex
	stopChan     chan struct{}
	flushTrigger chan struct{}
}

// Config holds event buffer configuration
type Config struct {
	Client      *httpclient.Client
	Endpoint    string
	BufferDir   string
	MaxSize     int
	FlushSize   int
	FlushPeriod time.Duration
}

// NewEventBuffer creates a new event buffer
func NewEventBuffer(cfg Config) (*EventBuffer, error) {
	if cfg.MaxSize == 0 {
		cfg.MaxSize = defaultBufferSize
	}
	if cfg.FlushSize == 0 {
		cfg.FlushSize = defaultFlushSize
	}
	if cfg.FlushPeriod == 0 {
		cfg.FlushPeriod = defaultFlushPeriod
	}

	// Create buffer directory
	if cfg.BufferDir == "" {
		cfg.BufferDir = filepath.Join(os.Getenv("ProgramData"), "MonitoringAgent", "buffer")
	}
	if err := os.MkdirAll(cfg.BufferDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create buffer directory: %w", err)
	}

	bufferFile := filepath.Join(cfg.BufferDir, "events.json")

	eb := &EventBuffer{
		client:       cfg.Client,
		endpoint:     cfg.Endpoint,
		bufferFile:   bufferFile,
		maxSize:      cfg.MaxSize,
		flushSize:    cfg.FlushSize,
		flushPeriod:  cfg.FlushPeriod,
		buffer:       make([]Event, 0, cfg.MaxSize),
		stopChan:     make(chan struct{}),
		flushTrigger: make(chan struct{}, 1),
	}

	// Load existing buffered events from disk
	if err := eb.loadFromDisk(); err != nil {
		log.Printf("Warning: failed to load buffered events: %v", err)
	}

	return eb, nil
}

// Start begins periodic flushing
func (eb *EventBuffer) Start(ctx context.Context) {
	ticker := time.NewTicker(eb.flushPeriod)
	defer ticker.Stop()
	defer eb.saveOnShutdown()

	for {
		select {
		case <-ctx.Done():
			log.Println("Event buffer shutting down...")
			return
		case <-eb.stopChan:
			log.Println("Event buffer stop signal received")
			return
		case <-ticker.C:
			eb.Flush(ctx)
		case <-eb.flushTrigger:
			eb.Flush(ctx)
		}
	}
}

// Stop stops the buffer
func (eb *EventBuffer) Stop() {
	close(eb.stopChan)
}

// Add adds an event to buffer
func (eb *EventBuffer) Add(eventType string, data interface{}) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	event := Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      jsonData,
	}

	// Add to buffer
	eb.buffer = append(eb.buffer, event)

	// Save to disk if buffer is getting large
	if len(eb.buffer) >= eb.maxSize/2 {
		if err := eb.saveToDisk(); err != nil {
			log.Printf("Warning: failed to save buffer to disk: %v", err)
		}
	}

	// Trigger flush if buffer is full
	if len(eb.buffer) >= eb.flushSize {
		select {
		case eb.flushTrigger <- struct{}{}:
		default:
		}
	}

	return nil
}

// Flush sends buffered events to server
func (eb *EventBuffer) Flush(ctx context.Context) error {
	eb.mu.Lock()
	if len(eb.buffer) == 0 {
		eb.mu.Unlock()
		return nil
	}

	// Take snapshot of current buffer
	eventsToSend := make([]Event, len(eb.buffer))
	copy(eventsToSend, eb.buffer)
	eb.mu.Unlock()

	// Try to send events
	payload := map[string]interface{}{
		"events": eventsToSend,
	}

	err := eb.client.PostJSON(ctx, eb.endpoint, payload)
	if err != nil {
		log.Printf("Failed to flush events to server: %v", err)
		// Save to disk for later retry
		eb.mu.Lock()
		if err := eb.saveToDisk(); err != nil {
			log.Printf("Failed to save buffer to disk: %v", err)
		}
		eb.mu.Unlock()
		return err
	}

	// Success - clear buffer
	eb.mu.Lock()
	eb.buffer = eb.buffer[:0]
	eb.mu.Unlock()

	// Remove buffer file
	if err := os.Remove(eb.bufferFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove buffer file: %v", err)
	}

	log.Printf("Successfully flushed %d events to server", len(eventsToSend))
	return nil
}

// Size returns current buffer size
func (eb *EventBuffer) Size() int {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	return len(eb.buffer)
}

// saveToDisk saves buffer to disk (called with lock held)
func (eb *EventBuffer) saveToDisk() error {
	if len(eb.buffer) == 0 {
		return nil
	}

	data, err := json.Marshal(eb.buffer)
	if err != nil {
		return fmt.Errorf("failed to marshal buffer: %w", err)
	}

	if err := os.WriteFile(eb.bufferFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write buffer file: %w", err)
	}

	return nil
}

// loadFromDisk loads buffer from disk
func (eb *EventBuffer) loadFromDisk() error {
	data, err := os.ReadFile(eb.bufferFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read buffer file: %w", err)
	}

	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return fmt.Errorf("failed to unmarshal buffer: %w", err)
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Limit loaded events to maxSize
	if len(events) > eb.maxSize {
		events = events[len(events)-eb.maxSize:]
	}

	eb.buffer = events
	log.Printf("Loaded %d buffered events from disk", len(events))

	return nil
}

// saveOnShutdown saves buffer to disk before shutdown
func (eb *EventBuffer) saveOnShutdown() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if len(eb.buffer) == 0 {
		log.Println("Event buffer is empty, nothing to save")
		return
	}

	// Try to flush one last time with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Unlock during network call
	eb.mu.Unlock()
	err := eb.Flush(ctx)
	eb.mu.Lock()

	if err == nil {
		log.Println("Successfully flushed events before shutdown")
		return
	}

	// Server unavailable - save to disk
	log.Printf("Server unavailable, saving %d events to disk", len(eb.buffer))
	if err := eb.saveToDisk(); err != nil {
		log.Printf("ERROR: Failed to save buffer to disk: %v", err)
	} else {
		log.Printf("Successfully saved %d events to %s", len(eb.buffer), eb.bufferFile)
	}
}
