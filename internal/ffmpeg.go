package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// GenerateSegment creates a segment file on disk (original implementation).
func GenerateSegment(ctx context.Context, videoPath, outputPath string, startSec, durationSec int) error {
	gop := strconv.Itoa(durationSec * 24)
	start := strconv.Itoa(startSec)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-hide_banner", "-loglevel", "error",
		"-ss", start,
		"-i", videoPath,
		"-t", strconv.Itoa(durationSec),
		"-map", "0:v:0", "-map", "0:a:0?", "-sn", "-dn",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-profile:v", "main", "-level", "3.1", "-pix_fmt", "yuv420p",
		"-g", gop, "-keyint_min", gop, "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000",
		"-af", "aresample=async=1:first_pts=0",
		"-output_ts_offset", start,
		"-f", "mpegts", "-muxdelay", "0", "-muxpreload", "0",
		outputPath,
	)
	return cmd.Run()
}

// GenerateSegmentCRF creates a segment with CRF quality control.
// CRF range: 15-30 recommended (lower = better quality, higher = smaller file)
func GenerateSegmentCRFOnBackground(ctx context.Context, videoPath, outputPath string, startSec, durationSec, crf int) error {
	// Validate CRF range
	if crf < 15 || crf > 30 {
		fmt.Printf("[ffmpeg] WARNING: CRF=%d is outside recommended range 15-30\n", crf)
	}

	// Use background context so FFmpeg isn't killed if client disconnects
	// The segment will be cached for future requests
	ctx = context.Background()

	// legge variabile d ambiene per capire se siamo su rpi
	swVideoEncoder := "libx264"
	hwVideoEncoder := "h264_v4l2m2m"
	encoder := ""
	if isRPI := os.Getenv("GAZEPARTY_RPI"); isRPI == "1" {
		encoder = hwVideoEncoder
	} else {
		encoder = swVideoEncoder
	}

	gop := strconv.Itoa(durationSec * 24)
	start := strconv.Itoa(startSec)

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-hide_banner", "-loglevel", "error",
		"-ss", start,
		"-i", videoPath,
		"-t", strconv.Itoa(durationSec),
		"-map", "0:v:0", "-map", "0:a:0?", "-sn", "-dn",
		"-c:v", encoder,
		"-preset", "ultrafast", "-tune", "zerolatency",
		"-crf", strconv.Itoa(crf),
		"-profile:v", "main", "-level", "3.1", "-pix_fmt", "yuv420p",
		"-g", gop, "-keyint_min", gop, "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000",
		"-af", "aresample=async=1:first_pts=0",
		"-output_ts_offset", start,
		"-f", "mpegts", "-muxdelay", "0", "-muxpreload", "0",
		outputPath,
	)
	return cmd.Run()
}

func GenerateSegmentV3(ctx context.Context, videoPath, outputPath string, startSec, durationSec, crf int) error {
	// Validate CRF range
	if crf < 15 || crf > 30 {
		fmt.Printf("[ffmpeg] WARNING: CRF=%d is outside recommended range 15-30\n", crf)
	}

	// Use background context so FFmpeg isn't killed if client disconnects
	// The segment will be cached for future requests
	ctx = context.Background()

	// Seek veloce a 10 sec prima, poi preciso
	preSeek := max(0, startSec-10)
	preciseSeek := startSec - preSeek

	gop := strconv.Itoa(durationSec * 24)
	start := strconv.Itoa(startSec)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-hide_banner", "-loglevel", "error",
		"-ss", strconv.Itoa(preSeek),
		"-i", videoPath,
		"-ss", strconv.Itoa(preciseSeek),
		"-t", strconv.Itoa(durationSec),
		"-map", "0:v:0", "-map", "0:a:0?", "-sn", "-dn",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency",
		"-crf", strconv.Itoa(crf),
		"-profile:v", "main", "-level", "3.1", "-pix_fmt", "yuv420p",
		"-g", gop, "-keyint_min", gop, "-sc_threshold", "0",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000",
		"-af", "aresample=async=1:first_pts=0",
		"-output_ts_offset", start,
		"-f", "mpegts", "-muxdelay", "0", "-muxpreload", "0",
		outputPath,
	)
	return cmd.Run()
}
