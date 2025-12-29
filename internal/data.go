package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// VideoData represents video info stored in the data file.
// This file tracks only base video information (hash, path, name, resolution, duration).
const dataDir = "/data"
const dataFile = "/data/videos.json"

type VideoData struct {
	ID       string  `json:"id"`
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Duration float64 `json:"duration"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
}

var (
	videoCache []VideoData
	cacheMu    sync.RWMutex
)

// LoadAndSyncVideos reads the data file, scans for new videos, and updates the file.
// - New videos are added with metadata extraction
// - Moved/renamed videos keep their Infos but update path/name
// - Deleted videos (hash not found) are removed
func LoadAndSyncVideos() ([]VideoData, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	// Scan all video files and compute hashes
	type scannedFile struct {
		path string
		hash string
	}
	var scanned []scannedFile
	filepath.Walk(videoDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && isVideo(info.Name()) {
			fmt.Printf("[data] hashing: %s\n", path)
			hash, err := fileHash(path)
			if err != nil {
				fmt.Printf("[data] error hashing %s: %v\n", path, err)
				return nil
			}
			scanned = append(scanned, scannedFile{path: path, hash: hash})
		}
		return nil
	})

	// Build map of current hashes -> path
	currentFiles := make(map[string]string) // hash -> path
	for _, f := range scanned {
		currentFiles[f.hash] = f.path
	}

	// Load existing data
	existing := loadDataFile()

	var result []VideoData
	var added, removed, updated int

	// Process existing entries: keep if hash still exists, update metadata if changed
	for _, v := range existing {
		if newPath, found := currentFiles[v.ID]; found {
			// Always refresh metadata (path, name, duration, resolution)
			duration, _ := videoDuration(newPath)
			width, height, _ := videoResolution(newPath)

			if v.Path != newPath || v.Duration != duration || v.Width != width || v.Height != height {
				fmt.Printf("[data] updated: %s\n", newPath)
				updated++
			}

			result = append(result, VideoData{
				ID:       v.ID,
				Path:     newPath,
				Name:     videoTitle(newPath),
				Duration: duration,
				Width:    width,
				Height:   height,
			})
			delete(currentFiles, v.ID) // mark as processed
		} else {
			fmt.Printf("[data] removed (file deleted): %s\n", v.Path)
			removed++
		}
	}

	// Add new videos (hashes not in existing data)
	for hash, path := range currentFiles {
		fmt.Printf("[data] new video: %s\n", path)
		duration, _ := videoDuration(path)
		width, height, _ := videoResolution(path)
		result = append(result, VideoData{
			ID:       hash,
			Path:     path,
			Name:     videoTitle(path),
			Duration: duration,
			Width:    width,
			Height:   height,
		})
		added++
	}

	// Always save and log stats
	fmt.Printf("[data] sync: added=%d removed=%d moved=%d\n", added, removed, updated)
	if err := saveDataFile(result); err != nil {
		fmt.Printf("[data] error saving: %v\n", err)
	}

	videoCache = result
	return result, nil
}

// GetVideos returns the cached video list.
func GetVideos() []VideoData {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return videoCache
}

// GetVideoByID returns a video by its ID (hash).
func GetVideoByID(id string) *VideoData {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	for i := range videoCache {
		if videoCache[i].ID == id {
			return &videoCache[i]
		}
	}
	return nil
}

func loadDataFile() []VideoData {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("[data] error reading file: %v\n", err)
		}
		return nil
	}

	var videos []VideoData
	if err := json.Unmarshal(data, &videos); err != nil {
		fmt.Printf("[data] error parsing file: %v\n", err)
		return nil
	}

	fmt.Printf("[data] loaded %d videos from file\n", len(videos))
	return videos
}

func saveDataFile(videos []VideoData) error {
	data, err := json.MarshalIndent(videos, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0644)
}
