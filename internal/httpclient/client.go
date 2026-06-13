package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client wraps HTTP calls with retries and JSON helpers.
type Client struct {
	HTTP       *http.Client
	MaxRetries int
	Backoff    time.Duration
}

// New creates a configured HTTP client.
func New(timeout time.Duration, maxRetries int, backoffMS int) *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: timeout,
		},
		MaxRetries: maxRetries,
		Backoff:    time.Duration(backoffMS) * time.Millisecond,
	}
}

// DoJSON performs a JSON request with retries on 429 and 5xx responses.
func (c *Client) DoJSON(ctx context.Context, method, url string, headers map[string]string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	var lastErr error
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(c.Backoff * time.Duration(attempt)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 300))
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if secs, err := time.ParseDuration(retryAfter + "s"); err == nil {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(secs):
					}
				}
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 500))
		}

		if out == nil {
			return nil
		}
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w (body: %s)", err, truncate(string(respBody), 300))
		}
		return nil
	}

	return fmt.Errorf("request failed after retries: %w", lastErr)
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
