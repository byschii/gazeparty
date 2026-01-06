package internal

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

const (
	videoDir        = "/video"
	segmentDuration = 4
)

// segmentLocks prevents concurrent encoding of the same segment
var (
	segmentLocks   = make(map[string]*sync.Mutex)
	segmentLocksMu sync.Mutex
)

// getSegmentLock returns a mutex for a specific segment (video_id + segment_num)
func getSegmentLock(key string) *sync.Mutex {
	segmentLocksMu.Lock()
	defer segmentLocksMu.Unlock()
	if segmentLocks[key] == nil {
		segmentLocks[key] = &sync.Mutex{}
	}
	return segmentLocks[key]
}

// GET /files
func HandleFiles(c *gin.Context) {
	c.JSON(200, GetVideos())
}

// GET /stream/:id/playlist.m3u8
func HandlePlaylist(c *gin.Context) {
	video := GetVideoByID(c.Param("id"))
	if video == nil {
		c.String(404, "video not found")
		return
	}

	numSegments := int(video.Duration/segmentDuration) + 1
	fmt.Printf("[playlist] path=%s duration=%.1fs segments=%d\n", video.Path, video.Duration, numSegments)

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")
	b.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", segmentDuration))
	b.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")

	for i := 0; i < numSegments; i++ {
		segDur := float64(segmentDuration)
		if i == numSegments-1 {
			segDur = video.Duration - float64(i*segmentDuration)
		}
		b.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", segDur))
		b.WriteString(fmt.Sprintf("segment_%d.ts\n", i))
	}
	b.WriteString("#EXT-X-ENDLIST\n")

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.String(200, b.String())
}

// GET /stream/:id/segment_:n.ts
func HandleSegment(c *gin.Context) {
	id := c.Param("id")
	video := GetVideoByID(id)
	if video == nil {
		c.String(404, "video not found")
		return
	}

	segStr := c.Param("n")
	segStr = strings.TrimPrefix(segStr, "segment_")
	segStr = strings.TrimSuffix(segStr, ".ts")
	segNum, err := strconv.Atoi(segStr)
	if err != nil {
		c.String(400, "invalid segment")
		return
	}

	// Segment file path
	segmentPath := fmt.Sprintf("/tmp/segments/%s/segment_%d.ts", id, segNum)
	segmentKey := fmt.Sprintf("%s_%d", id, segNum)

	// Lock this segment to prevent concurrent encoding
	lock := getSegmentLock(segmentKey)
	lock.Lock()
	defer lock.Unlock()

	// Check if already exists (after acquiring lock)
	if _, err := os.Stat(segmentPath); err != nil {
		// Create directory
		os.MkdirAll(fmt.Sprintf("/tmp/segments/%s", id), 0755)

		// Generate segment with CRF
		startTime := segNum * segmentDuration
		crf := 23
		fmt.Printf("[segment] generating seg=%d start=%ds crf=%d\n", segNum, startTime, crf)

		if err := GenerateSegmentCRFOnBackground(c.Request.Context(), video.Path, segmentPath, startTime, segmentDuration, crf); err != nil {
			fmt.Printf("[segment] error: %v\n", err)
			c.String(500, "ffmpeg error")
			return
		}
	}

	// Prefetch next 2 segments in background
	go prefetchSegments(video, segNum, 2)

	c.Header("Cache-Control", "public, max-age=3600")
	c.File(segmentPath)
}

// prefetchSegments encodes the next N segments in background
func prefetchSegments(video *VideoData, currentSeg, count int) {
	numSegments := int(video.Duration/segmentDuration) + 1
	crf := 23

	for i := 1; i <= count; i++ {
		nextSeg := currentSeg + i
		if nextSeg >= numSegments {
			break
		}

		segmentPath := fmt.Sprintf("/tmp/segments/%s/segment_%d.ts", video.ID, nextSeg)
		segmentKey := fmt.Sprintf("%s_%d", video.ID, nextSeg)

		// Try to acquire lock - if already locked, someone else is encoding it
		lock := getSegmentLock(segmentKey)
		if !lock.TryLock() {
			fmt.Printf("[prefetch] seg=%d already being encoded, skipping\n", nextSeg)
			continue
		}

		// Check if already exists (after acquiring lock)
		if _, err := os.Stat(segmentPath); err == nil {
			lock.Unlock()
			continue
		}

		startTime := nextSeg * segmentDuration
		fmt.Printf("[prefetch] generating seg=%d start=%ds\n", nextSeg, startTime)

		if err := GenerateSegmentCRFOnBackground(context.Background(), video.Path, segmentPath, startTime, segmentDuration, crf); err != nil {
			fmt.Printf("[prefetch] error seg=%d: %v\n", nextSeg, err)
		}
		lock.Unlock()
	}
}
