package main

import (
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// IngestStats tracks lightweight counters describing the ingest pipeline
// health per event type and per client. It is intentionally in-memory and
// resets on restart; persistence is delegated to ClickHouse for event data
// and to agent_configs for long-lived agent metadata.
type IngestStats struct {
	mu        sync.RWMutex
	perType   map[string]*typeCounter
	perClient map[string]map[string]time.Time // computer_name -> event_type -> last success
}

type typeCounter struct {
	Accepted      int64     `json:"accepted"`
	ParseErrors   int64     `json:"parse_errors"`
	InsertErrors  int64     `json:"insert_errors"`
	Dropped       int64     `json:"dropped"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastErrorAt   time.Time `json:"last_error_at,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
}

var ingestStats = &IngestStats{
	perType:   make(map[string]*typeCounter),
	perClient: make(map[string]map[string]time.Time),
}

func (s *IngestStats) ensureType(eventType string) *typeCounter {
	c, ok := s.perType[eventType]
	if !ok {
		c = &typeCounter{}
		s.perType[eventType] = c
	}
	return c
}

// Accept records a successful ingest of `eventType` produced by `client`.
// `client` may be empty when the event has no per-client identity.
func (s *IngestStats) Accept(eventType, client string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.ensureType(eventType)
	c.Accepted++
	c.LastSuccessAt = time.Now()

	if client != "" {
		cm, ok := s.perClient[client]
		if !ok {
			cm = make(map[string]time.Time)
			s.perClient[client] = cm
		}
		cm[eventType] = c.LastSuccessAt
	}
}

// ParseError records a payload parse failure for `eventType`.
func (s *IngestStats) ParseError(eventType string, err error) {
	s.recordError(eventType, err, func(c *typeCounter) { c.ParseErrors++ })
}

// InsertError records a DB insertion failure for `eventType`.
func (s *IngestStats) InsertError(eventType string, err error) {
	s.recordError(eventType, err, func(c *typeCounter) { c.InsertErrors++ })
}

// Drop records events that were discarded due to validation rules (e.g.
// missing required fields). They are not errors but useful to surface.
func (s *IngestStats) Drop(eventType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureType(eventType).Dropped++
}

func (s *IngestStats) recordError(eventType string, err error, update func(*typeCounter)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.ensureType(eventType)
	update(c)
	c.LastErrorAt = time.Now()
	if err != nil {
		c.LastError = err.Error()
	}
}

type clientSummary struct {
	ComputerName   string            `json:"computer_name"`
	LastSuccessAt  time.Time         `json:"last_success_at"`
	PerEventTypeAt map[string]string `json:"per_event_type_at"`
}

// Snapshot returns a point-in-time serializable snapshot of the counters.
func (s *IngestStats) Snapshot() gin.H {
	s.mu.RLock()
	defer s.mu.RUnlock()

	perType := make(map[string]typeCounter, len(s.perType))
	for k, v := range s.perType {
		perType[k] = *v
	}

	clients := make([]clientSummary, 0, len(s.perClient))
	for name, events := range s.perClient {
		summary := clientSummary{
			ComputerName:   name,
			PerEventTypeAt: make(map[string]string, len(events)),
		}
		for evt, ts := range events {
			summary.PerEventTypeAt[evt] = ts.Format(time.RFC3339)
			if ts.After(summary.LastSuccessAt) {
				summary.LastSuccessAt = ts
			}
		}
		clients = append(clients, summary)
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].LastSuccessAt.After(clients[j].LastSuccessAt)
	})

	return gin.H{
		"per_type": perType,
		"clients":  clients,
	}
}

// ingestStatsHandler exposes the counters over HTTP for operational probes.
func ingestStatsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, ingestStats.Snapshot())
}
