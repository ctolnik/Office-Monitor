package main

import (
	"context"
	"sync"
	"time"

	"github.com/ctolnik/Office-Monitor/zapctx"
	"go.uber.org/zap"
)

// heartbeatThrottle defines the minimum interval between consecutive
// agent_configs upserts for the same computer. It keeps heartbeat traffic
// cheap even when agents send events every few seconds.
const heartbeatThrottle = 60 * time.Second

// heartbeatFlushInterval controls how often the background flusher promotes
// in-memory ingest activity into agent_configs rows. It is coarser than the
// throttle to avoid a dedicated write for every incoming batch while still
// keeping `last_seen` up to date for dashboards.
const heartbeatFlushInterval = 30 * time.Second

// heartbeatStaleWindow scopes the background flusher to clients that have
// produced ingest traffic recently. Stale entries are skipped so a single
// restart burst cannot amplify writes indefinitely.
const heartbeatStaleWindow = 5 * time.Minute

var (
	heartbeatMu    sync.Mutex
	lastHeartbeats = make(map[string]time.Time)
)

// recordHeartbeat writes a throttled agent_configs upsert for `computer`.
// The helper is safe to call from any request handler; it performs the
// ClickHouse write inline only when the last recorded heartbeat is older
// than `heartbeatThrottle`. Errors are logged and swallowed so the caller's
// primary response is never affected.
func recordHeartbeat(ctx context.Context, computer string) {
	if computer == "" || db == nil {
		return
	}

	now := time.Now()
	heartbeatMu.Lock()
	last := lastHeartbeats[computer]
	if !last.IsZero() && now.Sub(last) < heartbeatThrottle {
		heartbeatMu.Unlock()
		return
	}
	lastHeartbeats[computer] = now
	heartbeatMu.Unlock()

	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.UpdateAgentHeartbeat(writeCtx, computer); err != nil {
		zapctx.Warn(ctx, "Failed to update agent heartbeat",
			zap.String("computer", computer),
			zap.Error(err),
		)
	}
}

// heartbeatFlusher periodically syncs in-memory ingest activity to
// agent_configs. It complements recordHeartbeat by ensuring long-running
// clients do not accidentally drop out of the heartbeat table between
// throttle windows, and by surfacing clients reported via handlers that do
// not call recordHeartbeat directly.
func heartbeatFlusher(ctx context.Context) {
	ticker := time.NewTicker(heartbeatFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			flushHeartbeats(ctx)
		}
	}
}

func flushHeartbeats(ctx context.Context) {
	if db == nil {
		return
	}
	snapshot := ingestStats.Snapshot()
	clientsAny, ok := snapshot["clients"].([]clientSummary)
	if !ok {
		return
	}

	threshold := time.Now().Add(-heartbeatStaleWindow)
	for _, client := range clientsAny {
		if client.ComputerName == "" || client.LastSuccessAt.Before(threshold) {
			continue
		}
		recordHeartbeat(ctx, client.ComputerName)
	}
}
