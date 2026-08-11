package fetcher

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxFetchAttempts = 4
	defaultRetryWait = 2 * time.Second
	maxRetryWait     = 30 * time.Second
)

var client = &http.Client{
	Timeout: 17 * time.Second,
}

// https://create.roblox.com/docs/cloud/reference/rate-limits
var typeInterval = map[string]time.Duration{
	"badges":         500 * time.Millisecond,
	"games":          500 * time.Millisecond,
	"favorites":      500 * time.Millisecond,
	"votes":          500 * time.Millisecond,
	"virtual-events": 500 * time.Millisecond,
	"users":          100 * time.Millisecond,
	"groups":         100 * time.Millisecond,
	"universes":      100 * time.Millisecond,
	"places":         100 * time.Millisecond,
}

type typeLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	last     time.Time
}

var (
	limitersMu sync.Mutex
	limiters   = map[string]*typeLimiter{}
)

func limiterFor(endpointType string) *typeLimiter {
	limitersMu.Lock()
	defer limitersMu.Unlock()
	if lim, ok := limiters[endpointType]; ok {
		return lim
	}
	interval, ok := typeInterval[endpointType]
	if !ok {
		interval = 250 * time.Millisecond
	}
	lim := &typeLimiter{interval: interval}
	limiters[endpointType] = lim
	return lim
}

func (l *typeLimiter) wait() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if !l.last.IsZero() {
		if wait := l.interval - now.Sub(l.last); wait > 0 {
			time.Sleep(wait)
			now = time.Now()
		}
	}
	l.last = now
}

func FetchForEndpoint(endpointType, url string, headers map[string]string) ([]byte, error) {
	limiterFor(endpointType).wait()

	var lastErr error
	for attempt := 1; attempt <= maxFetchAttempts; attempt++ {
		body, status, retryAfter, err := doFetch(url, headers)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable(status) || attempt == maxFetchAttempts {
			return nil, err
		}

		wait := retryAfter
		if wait <= 0 {
			wait = defaultRetryWait << (attempt - 1)
		}
		if wait > maxRetryWait {
			wait = maxRetryWait
		}
		log.Printf("[WARN] fetcher: %s attempt %d/%d failed (%v); retrying in %v",
			endpointType, attempt, maxFetchAttempts, err, wait)
		time.Sleep(wait)
		limiterFor(endpointType).wait()
	}
	return nil, lastErr
}

func retryable(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

func doFetch(url string, headers map[string]string) ([]byte, int, time.Duration, error) {
	var (
		resp *http.Response
		err  error
	)
	if len(headers) == 0 {
		resp, err = client.Get(url)
	} else {
		req, reqErr := http.NewRequest(http.MethodGet, url, nil)
		if reqErr != nil {
			return nil, 0, 0, reqErr
		}
		for k, v := range headers {
			if v != "" {
				req.Header.Set(k, v)
			}
		}
		resp, err = client.Do(req)
	}
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, resp.StatusCode, retryAfter, fmt.Errorf(
			"request failed (%d) for %s: %s",
			resp.StatusCode, url, strings.TrimSpace(string(body)),
		)
	}
	data, err := io.ReadAll(resp.Body)
	return data, resp.StatusCode, 0, err
}

func parseRetryAfter(raw string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || secs <= 0 {
		return 0
	}
	d := time.Duration(secs) * time.Second
	if d > maxRetryWait {
		return maxRetryWait
	}
	return d
}
