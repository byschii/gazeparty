package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const segmentsDir = "/tmp/segments"

// StartCleanup avvia un task in background che pulisce i segmenti vecchi
func StartCleanup(interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			cleanupOldSegments(maxAge)
		}
	}()
	fmt.Printf("[cleanup] started: interval=%v maxAge=%v\n", interval, maxAge)
}

func cleanupOldSegments(maxAge time.Duration) {
	now := time.Now()
	removed := 0

	filepath.Walk(segmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is older than maxAge
		if now.Sub(info.ModTime()) > maxAge {
			if err := os.Remove(path); err == nil {
				removed++
			}
		}

		return nil
	})

	// Remove empty directories
	filepath.Walk(segmentsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || path == segmentsDir {
			return nil
		}

		// Try to remove (will fail if not empty)
		os.Remove(path)
		return nil
	})

	if removed > 0 {
		fmt.Printf("[cleanup] removed %d old segments\n", removed)
	}
}
