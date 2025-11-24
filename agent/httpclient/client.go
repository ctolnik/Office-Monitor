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

	"github.com/google/uuid"
	"github.com/sony/gobreaker"
)

// Client represents an HTTP client with retry logic, circuit breaker, and authentication
type Client struct {
	serverURL      string
	apiKey         string
	httpClient     *http.Client
	retryAttempts  int
	retryDelay     time.Duration
	circuitBreaker *gobreaker.CircuitBreaker
}

// Config holds configuration for the HTTP client
type Config struct {
	ServerURL      string
	APIKey         string
	TimeoutSeconds int
	RetryAttempts  int
	RetryDelay     time.Duration
}

// NewClient creates a new HTTP client with circuit breaker
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

	// Configure circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "MonitoringServerAPI",
		MaxRequests: 3,                // Max requests allowed in half-open state
		Interval:    60 * time.Second, // Period to clear failure counts
		Timeout:     30 * time.Second, // Time to wait before half-open after open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker '%s' state changed: %s -> %s", name, from, to)
		},
	}

	return &Client{
		serverURL:      cfg.ServerURL,
		apiKey:         cfg.APIKey,
		retryAttempts:  cfg.RetryAttempts,
		retryDelay:     cfg.RetryDelay,
		circuitBreaker: gobreaker.NewCircuitBreaker(cbSettings),
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		},
	}
}

// PostJSON sends a POST request with JSON body (protected by circuit breaker)
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

		// Generate unique request ID for tracing
		requestID := uuid.New().String()
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", requestID)
		if c.apiKey != "" {
			req.Header.Set("X-API-Key", c.apiKey)
		}

		start := time.Now()

		// Execute request through circuit breaker
		resp, err := c.executeWithCircuitBreaker(req)
		duration := time.Since(start)

		if err != nil {
			lastErr = fmt.Errorf("[request_id=%s] request failed after %v: %w", requestID, duration, err)
			log.Printf("%v", lastErr)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Get server's request ID (may be same or server-generated)
		serverRequestID := resp.Header.Get("X-Request-ID")
		if serverRequestID != "" {
			requestID = serverRequestID
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[request_id=%s] POST %s succeeded (%d) after %v", requestID, endpoint, resp.StatusCode, duration)
			return nil
		}

		if resp.StatusCode >= 500 {
			// Server error - retry
			lastErr = fmt.Errorf("[request_id=%s] server error %d after %v: %s", requestID, resp.StatusCode, duration, string(body))
			log.Printf("%v", lastErr)
			continue
		}

		// Client error (4xx) - don't retry
		err = fmt.Errorf("[request_id=%s] client error %d after %v: %s", requestID, resp.StatusCode, duration, string(body))
		log.Printf("%v", err)
		return err
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

		// Execute request through circuit breaker
		resp, err := c.executeWithCircuitBreaker(req)
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

// executeWithCircuitBreaker wraps HTTP request execution with circuit breaker protection
func (c *Client) executeWithCircuitBreaker(req *http.Request) (*http.Response, error) {
	result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
		return c.httpClient.Do(req)
	})

	if err != nil {
		return nil, err
	}

	return result.(*http.Response), nil
}
