// Package accrual talks to the external accrual system and applies its
// results to pending orders.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultRetryAfter = 60 * time.Second

// ErrNotRegistered is returned when the accrual system does not know the order.
var ErrNotRegistered = errors.New("order is not registered in the accrual system")

// RateLimitError is returned when the accrual system answers 429. RetryAfter
// is how long the caller must wait before the next request.
type RateLimitError struct {
	RetryAfter time.Duration
}

// Error implements error.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("accrual system rate limit: retry after %s", e.RetryAfter)
}

// Result is the accrual system's answer for an order. Accrual is in points
// and is nil when the system reported none.
type Result struct {
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual"`
}

// Client is an HTTP client of the accrual system.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a Client for the accrual system at baseURL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetOrder asks the accrual system about the order. It returns
// ErrNotRegistered on 204 and *RateLimitError on 429.
func (c *Client) GetOrder(ctx context.Context, number string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/api/orders/"+url.PathEscape(number), nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("request accrual system: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var res Result
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			return Result{}, fmt.Errorf("decode accrual response: %w", err)
		}
		return res, nil
	case http.StatusNoContent:
		return Result{}, ErrNotRegistered
	case http.StatusTooManyRequests:
		return Result{}, &RateLimitError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	default:
		return Result{}, fmt.Errorf("accrual system returned status %d", resp.StatusCode)
	}
}

func parseRetryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs < 0 {
		return defaultRetryAfter
	}
	return time.Duration(secs) * time.Second
}
