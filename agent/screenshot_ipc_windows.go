//go:build windows
// +build windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ctolnik/Office-Monitor/agent/httpclient"
	"github.com/ctolnik/Office-Monitor/agent/pkg/ipc"
	"go.uber.org/zap"
)

type screenshotState struct {
	meta ipc.ScreenshotBegin
	buf  []byte
}

type ScreenshotAssembler struct {
	mu    sync.Mutex
	shots map[string]*screenshotState
	log   *zap.Logger
	cli   *httpclient.Client
}

func NewScreenshotAssembler(serverURL, apiKey, computerName string, log *zap.Logger) *ScreenshotAssembler {
	cli := httpclient.NewClient(httpclient.Config{
		ServerURL:      serverURL,
		APIKey:         apiKey,
		TimeoutSeconds: 120,
		RetryAttempts:  3,
	})
	return &ScreenshotAssembler{
		shots: make(map[string]*screenshotState),
		log:   log.With(zap.String("component", "screenshot_assembler"), zap.String("computer_name", computerName)),
		cli:   cli,
	}
}

func (a *ScreenshotAssembler) HandleBegin(e ipc.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	b, _ := json.Marshal(e.Data)
	var meta ipc.ScreenshotBegin
	if err := json.Unmarshal(b, &meta); err != nil {
		return err
	}
	if meta.ScreenshotID == "" || meta.TotalSize <= 0 {
		return fmt.Errorf("invalid screenshot_begin")
	}

	a.shots[meta.ScreenshotID] = &screenshotState{
		meta: meta,
		buf:  make([]byte, meta.TotalSize),
	}
	a.log.Info("Screenshot begin",
		zap.String("screenshot_id", meta.ScreenshotID),
		zap.Int("total_size", meta.TotalSize),
		zap.String("username", meta.Username),
		zap.String("session_id", meta.SessionID),
	)
	return nil
}

func (a *ScreenshotAssembler) HandleChunk(e ipc.Event) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	b, _ := json.Marshal(e.Data)
	var chunk ipc.ScreenshotChunk
	if err := json.Unmarshal(b, &chunk); err != nil {
		return err
	}

	st, ok := a.shots[chunk.ScreenshotID]
	if !ok {
		return fmt.Errorf("unknown screenshot_id: %s", chunk.ScreenshotID)
	}

	if chunk.Offset < 0 || chunk.Offset+len(chunk.Data) > len(st.buf) {
		return fmt.Errorf("chunk out of bounds")
	}
	copy(st.buf[chunk.Offset:], chunk.Data)
	return nil
}

func (a *ScreenshotAssembler) HandleCommit(e ipc.Event) error {
	// copy out state then do upload without holding the lock
	a.mu.Lock()
	b, _ := json.Marshal(e.Data)
	var commit ipc.ScreenshotCommit
	if err := json.Unmarshal(b, &commit); err != nil {
		a.mu.Unlock()
		return err
	}

	st, ok := a.shots[commit.ScreenshotID]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("unknown screenshot_id: %s", commit.ScreenshotID)
	}
	delete(a.shots, commit.ScreenshotID)
	a.mu.Unlock()

	sum := sha256.Sum256(st.buf)
	got := hex.EncodeToString(sum[:])
	if st.meta.SHA256 != "" && got != st.meta.SHA256 {
		a.log.Warn("Screenshot sha256 mismatch",
			zap.String("screenshot_id", st.meta.ScreenshotID),
			zap.String("expected", st.meta.SHA256),
			zap.String("got", got),
		)
		return fmt.Errorf("sha256 mismatch")
	}

	payload := map[string]interface{}{
		"timestamp":     st.meta.Timestamp,
		"computer_name": st.meta.ComputerName,
		"username":      st.meta.Username,
		"screenshot_id": st.meta.ScreenshotID,
		"window_title":  st.meta.WindowTitle,
		"process_name":  st.meta.ProcessName,
		"file_size":     int64(len(st.buf)),
		"image_data":    st.buf,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := a.cli.PostJSON(ctx, "/api/screenshot", payload); err != nil {
		a.log.Error("Failed to upload screenshot",
			zap.String("screenshot_id", st.meta.ScreenshotID),
			zap.Error(err),
		)
		return err
	}

	a.log.Info("Screenshot uploaded",
		zap.String("screenshot_id", st.meta.ScreenshotID),
		zap.Int("size", len(st.buf)),
	)
	return nil
}

// compile-time guard to ensure we don't accidentally use bytes package unused
var _ = bytes.NewBuffer
