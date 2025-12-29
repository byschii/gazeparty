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

// LoadAndSyncVideos scans videos in parallel (4 workers), syncs with data file.
func LoadAndSyncVideos() ([]VideoData, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	// Collect paths
	var paths []string
	filepath.Walk(videoDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && isVideo(info.Name()) {
			paths = append(paths, path)
		}
		return nil
	})

	// Process files in parallel (4 workers)
	results := make(chan VideoData, len(paths))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fmt.Printf("[data] processing: %s\n", p)
			hash, err := fileHash(p)
			if err != nil {
				fmt.Printf("[data] error hashing %s: %v\n", p, err)
				return
			}
			duration, _ := videoDuration(p)
			width, height, _ := videoResolution(p)
			results <- VideoData{ID: hash, Path: p, Name: videoTitle(p), Duration: duration, Width: width, Height: height}
		}(path)
	}

	go func() { wg.Wait(); close(results) }()

	// Build map from scanned files
	scanned := make(map[string]VideoData)
	for v := range results {
		scanned[v.ID] = v
	}

	// Reconcile with existing data
	existing := loadDataFile()
	existingMap := make(map[string]VideoData)
	for _, v := range existing {
		existingMap[v.ID] = v
	}

	var result []VideoData
	var added, removed, updated int

	for id, v := range scanned {
		if old, found := existingMap[id]; found {
			if old.Path != v.Path || old.Duration != v.Duration || old.Width != v.Width || old.Height != v.Height {
				fmt.Printf("[data] updated: %s\n", v.Path)
				updated++
			}
		} else {
			fmt.Printf("[data] new: %s\n", v.Path)
			added++
		}
		result = append(result, v)
	}

	for id, v := range existingMap {
		if _, found := scanned[id]; !found {
			fmt.Printf("[data] removed: %s\n", v.Path)
			removed++
		}
	}

	fmt.Printf("[data] sync: added=%d removed=%d updated=%d\n", added, removed, updated)
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
