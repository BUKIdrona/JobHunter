package source

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

const userAgent = "jobhunter/1.0 (personal job search; uses public job feeds)"

var client = &http.Client{Timeout: 25 * time.Second}

func getBytes(ctx context.Context, rawURL string) ([]byte, error) {
	return getBytesHeaders(ctx, rawURL, nil)
}

func getBytesHeaders(ctx context.Context, rawURL string, headers map[string]string) ([]byte, error) {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			last = err
			sleep(ctx, time.Second)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 15<<20))
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			last = fmt.Errorf("%s: %s", rawURL, resp.Status)
			sleep(ctx, 1500*time.Millisecond)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", rawURL, resp.Status)
		}
		return body, nil
	}
	return nil, last
}

func getJSON(ctx context.Context, rawURL string, dest any) error {
	body, err := getBytes(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("%s: %w", rawURL, err)
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexID(s)
		return nil
	}
	*f = flexID(b)
	return nil
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" || value == "null" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func parseUnix(n float64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	if n > 1e12 {
		return time.UnixMilli(int64(n)).UTC()
	}
	return time.Unix(int64(n), 0).UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func companyFromToken(token string) string {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "":
		return "Unknown"
	case "cred":
		return "CRED"
	default:
		return strings.ToUpper(token[:1]) + token[1:]
	}
}

func salaryRange(min, max float64, currency string) string {
	if min <= 0 && max <= 0 {
		return ""
	}
	cur := strings.TrimSpace(currency)
	switch {
	case min > 0 && max > 0:
		return fmt.Sprintf("%s %.0f-%.0f", cur, min, max)
	case min > 0:
		return fmt.Sprintf("%s %.0f+", cur, min)
	default:
		return fmt.Sprintf("up to %s %.0f", cur, max)
	}
}
