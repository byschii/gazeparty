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

// ...existing code...

// GenerateSegmentV4 creates a segment with proper handling for both software and hardware encoders.
// On Raspberry Pi (GAZEPARTY_RPI=1), uses h264_v4l2m2m with bitrate control.
// On other systems, uses libx264 with CRF quality control.
// bitrateMbps: target bitrate in Megabits (e.g., 3 for 3M), used only for hardware encoder.
func GenerateSegmentV4(ctx context.Context, videoPath, outputPath string, startSec, durationSec, crf, bitrateMbps int) error {
	// Use background context so FFmpeg isn't killed if client disconnects
	ctx = context.Background()

	// Seek veloce a 10 sec prima, poi preciso
	preSeek := max(0, startSec-10)
	preciseSeek := startSec - preSeek

	gop := strconv.Itoa(durationSec * 24)
	start := strconv.Itoa(startSec)

	// Base args comuni
	args := []string{
		"-y",
		"-hide_banner", "-loglevel", "error",
		"-ss", strconv.Itoa(preSeek),
		"-i", videoPath,
		"-ss", strconv.Itoa(preciseSeek),
		"-t", strconv.Itoa(durationSec),
		"-map", "0:v:0", "-map", "0:a:0?", "-sn", "-dn",
	}

	// Scegli encoder e parametri in base all'ambiente
	isRPI := os.Getenv("GAZEPARTY_RPI") == "1"

	if isRPI {
		// Hardware encoder: h264_v4l2m2m
		// - NO CRF support, usa bitrate
		// - NO preset/tune support
		// - GOP flags spesso ignorati
		// - Richiede pix_fmt yuv420p esplicito PRIMA dell'encoder
		if bitrateMbps <= 0 {
			bitrateMbps = 3 // Default 3 Mbps
		}
		bitrate := strconv.Itoa(bitrateMbps) + "M"

		args = append(args,
			"-pix_fmt", "yuv420p", // DEVE essere prima di -c:v per hw encoder
			"-c:v", "h264_v4l2m2m",
			"-b:v", bitrate,
			"-profile:v", "main",
		)
		fmt.Printf("[ffmpeg] Using h264_v4l2m2m hardware encoder @ %s bitrate\n", bitrate)
	} else {
		// Software encoder: libx264 con CRF
		if crf < 15 || crf > 30 {
			fmt.Printf("[ffmpeg] WARNING: CRF=%d is outside recommended range 15-30\n", crf)
		}

		args = append(args,
			"-c:v", "libx264",
			"-preset", "ultrafast", "-tune", "zerolatency",
			"-crf", strconv.Itoa(crf),
			"-profile:v", "main", "-level", "3.1",
			"-pix_fmt", "yuv420p",
			"-g", gop, "-keyint_min", gop, "-sc_threshold", "0",
		)
	}

	// Audio args (comuni a entrambi)
	args = append(args,
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "48000",
		"-af", "aresample=async=1:first_pts=0",
		"-output_ts_offset", start,
		"-f", "mpegts", "-muxdelay", "0", "-muxpreload", "0",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	return cmd.Run()
}
