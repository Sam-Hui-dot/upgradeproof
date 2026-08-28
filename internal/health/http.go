package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HTTPWaiter struct {
	Client *http.Client
}

func (w HTTPWaiter) Wait(ctx context.Context, url string, timeout, interval time.Duration) error {
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	var last string
	for {
		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			last = resp.Status
		} else {
			last = err.Error()
		}
		select {
		case <-deadlineCtx.Done():
			return fmt.Errorf("HTTP health timed out after %s (last result: %s): %w", timeout, last, deadlineCtx.Err())
		case <-ticker.C:
		}
	}
}
