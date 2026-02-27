// CLAUDE:SUMMARY Shared import utilities: HTTP download with retries, ZIP extraction, manifest YAML writer, directory helpers.
package importer

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hazyhaar/touchstone-registry/pkg/dict"
	"gopkg.in/yaml.v3"
)

// downloadFile downloads url to dest with retries and timeout.
func downloadFile(ctx context.Context, url, dest string) error {
	client := &http.Client{Timeout: 10 * time.Minute}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
			continue
		}

		f, err := os.Create(dest)
		if err != nil {
			resp.Body.Close()
			return fmt.Errorf("create file: %w", err)
		}

		_, copyErr := io.Copy(f, resp.Body)
		resp.Body.Close()
		closeErr := f.Close()

		if copyErr != nil {
			lastErr = copyErr
			continue
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	}
	return fmt.Errorf("download %s failed after 3 attempts: %w", url, lastErr)
}

// unzipFile extracts a ZIP archive to destDir and returns the list of extracted file paths.
func unzipFile(src, destDir string) ([]string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var paths []string
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		destPath := filepath.Join(destDir, filepath.Base(f.Name))
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}

		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return nil, fmt.Errorf("create %s: %w", destPath, err)
		}

		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return nil, fmt.Errorf("extract %s: %w", f.Name, err)
		}
		rc.Close()
		out.Close()
		paths = append(paths, destPath)
	}
	return paths, nil
}

// writeManifest writes a Manifest as YAML to dir/manifest.yaml.
func writeManifest(dir string, m *dict.Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, "manifest.yaml"), data, 0o644)
}

// ensureDir creates a directory if it doesn't exist.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
