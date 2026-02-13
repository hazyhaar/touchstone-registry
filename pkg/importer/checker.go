package importer

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Checker performs periodic HEAD requests against all registered import sources
// and logs their availability.
type Checker struct {
	sources  *SourceDB
	logger   *slog.Logger
	interval time.Duration
	client   *http.Client
}

// NewChecker creates a Checker that will verify source URLs every interval.
func NewChecker(sources *SourceDB, logger *slog.Logger, interval time.Duration) *Checker {
	return &Checker{
		sources:  sources,
		logger:   logger,
		interval: interval,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Start runs an immediate check then repeats every interval until ctx is cancelled.
func (c *Checker) Start(ctx context.Context) {
	c.CheckAll(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.CheckAll(ctx)
		}
	}
}

// CheckAll performs a HEAD request on every source URL and persists the result.
func (c *Checker) CheckAll(ctx context.Context) {
	sources, err := c.sources.ListSources()
	if err != nil {
		c.logger.Error("source check: impossible de lister les sources", "error", err)
		return
	}
	if len(sources) == 0 {
		return
	}

	var ok, failed int
	for _, src := range sources {
		if ctx.Err() != nil {
			return
		}

		status, checkErr := c.checkOne(ctx, src.SourceURL)
		errMsg := ""
		if checkErr != nil {
			errMsg = checkErr.Error()
		}

		if err := c.sources.UpdateCheck(src.AdapterID, status, errMsg); err != nil {
			c.logger.Error("source check: echec mise a jour", "adapter", src.AdapterID, "error", err)
		}

		if status >= 200 && status < 400 {
			ok++
		} else {
			failed++
			c.logger.Warn("source inaccessible",
				"adapter", src.AdapterID,
				"url", src.SourceURL,
				"status", status,
				"error", errMsg,
			)
		}
	}

	c.logger.Info("source check complete", "total", ok+failed, "ok", ok, "failed", failed)
}

// checkOne performs a single HEAD request and returns the HTTP status code.
// On network error, status is 0.
func (c *Checker) checkOne(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HEAD %s: %w", url, err)
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
