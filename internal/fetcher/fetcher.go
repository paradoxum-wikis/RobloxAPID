package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var client = &http.Client{
	Timeout: 17 * time.Second,
}

func Fetch(url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readResponseBody(url, resp)
}

func FetchWithHeaders(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return readResponseBody(url, resp)
}

func readResponseBody(url string, resp *http.Response) ([]byte, error) {
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("request failed (%d) for %s: %s", resp.StatusCode, url, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}
