package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Client represents an HTTP client with retry logic and authentication
type Client struct {
	serverURL     string
	apiKey        string
	httpClient    *http.Client
	retryAttempts int
	retryDelay    time.Duration
}

// Config holds configuration for the HTTP client
type Config struct {
	ServerURL      string
	APIKey         string
	TimeoutSeconds int
	RetryAttempts  int
	RetryDelay     time.Duration
}

// NewClient creates a new HTTP client
func NewClient(cfg Config) *Client {
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 30
	}
	if cfg.RetryAttempts == 0 {
		cfg.RetryAttempts = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 5 * time.Second
	}

	return &Client{
		serverURL:     cfg.ServerURL,
		apiKey:        cfg.APIKey,
		retryAttempts: cfg.RetryAttempts,
		retryDelay:    cfg.RetryDelay,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
	}
}

// PostJSON sends a POST request with JSON body
func (c *Client) PostJSON(ctx context.Context, endpoint string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	url := c.serverURL + endpoint

	var lastErr error
	for attempt := 0; attempt <= c.retryAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying request to %s (attempt %d/%d)", endpoint, attempt, c.retryAttempts)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryDelay):
				// Continue with retry
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		if resp.StatusCode >= 500 {
			// Server error - retry
			lastErr = fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
			continue
		}

		// Client error (4xx) - don't retry
		return fmt.Errorf("client error %d: %s", resp.StatusCode, string(body))
	}

	return fmt.Errorf("request failed after %d attempts: %w", c.retryAttempts, lastErr)
}

// PostMultipart sends a multipart/form-data request (for file uploads)
func (c *Client) PostMultipart(ctx context.Context, endpoint string, body io.Reader, contentType string) error {
	url := c.serverURL + endpoint

	var lastErr error
	for attempt := 0; attempt <= c.retryAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying multipart request to %s (attempt %d/%d)", endpoint, attempt, c.retryAttempts)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.retryDelay):
				// Continue with retry
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, body)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", contentType)
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error %d: %s", resp.StatusCode, string(respBody))
			continue
		}

		return fmt.Errorf("client error %d: %s", resp.StatusCode, string(respBody))
	}

	return fmt.Errorf("multipart request failed after %d attempts: %w", c.retryAttempts, lastErr)
}

// Ping checks if the server is reachable
func (c *Client) Ping(ctx context.Context) error {
	url := c.serverURL + "/health"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}
