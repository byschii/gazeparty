package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var videoExts = []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm"}

func isVideo(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	for _, v := range videoExts {
		if ext == v {
			return true
		}
	}
	return false
}

func videoDuration(path string) (float64, error) {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func videoResolution(path string) (int, int, error) {
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height",
		"-of", "csv=p=0",
		path,
	).Output()
	if err != nil {
		return 0, 0, err
	}

	// Trim spaces and trailing commas (some files output "1920,1080," instead of "1920,1080")
	output := strings.TrimSpace(string(out))
	output = strings.TrimSuffix(output, ",")

	parts := strings.Split(output, ",")
	if len(parts) != 2 {
		return 0, 0, nil
	}
	w, _ := strconv.Atoi(parts[0])
	h, _ := strconv.Atoi(parts[1])
	return w, h, nil
}

func videoTitle(path string) string {
	// Try to get title from metadata
	out, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format_tags=title",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	).Output()
	if err == nil {
		title := strings.TrimSpace(string(out))
		if title != "" {
			return title
		}
	}
	// Fallback to filename without extension
	return nameWithoutExt(path)
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

func nameWithoutExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}
