package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type FileInfo struct {
	Name   string
	Path   string
	IsDir  bool
	Hashed string
}

func HandleBrowse(c *gin.Context) {
	// Con NoRoute, prendiamo il path dall'URL completo
	requestedPath := c.Request.URL.Path
	if requestedPath == "" || requestedPath == "/" {
		requestedPath = "/video"
	}

	// Verifica se esiste
	info, err := os.Stat(requestedPath)
	if err != nil {
		c.String(404, "Non trovato")
		return
	}

	// Se è un file → mostra player
	if !info.IsDir() {
		if IsVideo(requestedPath) {
			c.HTML(200, "player.html", gin.H{
				"Path":     requestedPath,
				"Filename": filepath.Base(requestedPath),
			})
		} else {
			c.String(400, "Non è un video")
		}
		return
	}

	// Se è una directory → mostra explorer
	files, _ := os.ReadDir(requestedPath)

	var folders []FileInfo
	var videos []FileInfo

	for _, file := range files {

		/*hashed, err := SHA256File(filepath.Join(requestedPath, file.Name()))
		if err != nil {
			log.Printf("Errore calcolando hash per %s: %v", file.Name(), err)
			hashed = "Errore"
		}*/

		fileInfo := FileInfo{
			Name:   file.Name(),
			Path:   filepath.Join(requestedPath, file.Name()),
			IsDir:  file.IsDir(),
			Hashed: "",
		}

		if file.IsDir() {
			folders = append(folders, fileInfo)
		} else if IsVideo(file.Name()) {
			videos = append(videos, fileInfo)
		}
	}

	c.HTML(200, "browser.html", gin.H{
		"CurrentPath": requestedPath,
		"ParentPath":  filepath.Dir(requestedPath),
		"HasParent":   requestedPath != "/video",
		"Folders":     folders,
		"Videos":      videos,
	})
}

func HandleStream(c *gin.Context) {
	videoPath := c.Param("filepath")

	// Il path dal parametro già inizia con / (es: /video/film/surfsup.mkv)
	// Questo è già il path assoluto che ci serve

	// Verifica che il file esista
	if _, err := os.Stat(videoPath); err != nil {
		c.String(404, "File non trovato: %s", videoPath)
		return
	}

	c.Header("Content-Type", "video/mp4")

	if !checkNeedsTranscode(videoPath) {
		// Stream diretto con supporto Range automatico
		c.File(videoPath)
	} else {
		// Transcoding con supporto per seek
		rangeHeader := c.GetHeader("Range")

		var seekTime string
		if rangeHeader != "" {
			// Estrai il byte offset dal Range header
			// Calcola approssimativamente il tempo di seek
			seekTime = calculateSeekTime(videoPath, rangeHeader)
		}

		streamTranscodedVideo(c, videoPath, seekTime)
	}
}

func FindFromHash(c *gin.Context) {
	hashToFind := c.Query("hash")

	// Cerca nella directory /video
	err := filepath.Walk("/video", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && IsVideo(info.Name()) {
			fileHash, err := SHA256File(path)
			if err != nil {
				return err
			}

			if strings.HasPrefix(fileHash, hashToFind) {
				// Reindirizza al file trovato
				c.Redirect(302, path)
				return fmt.Errorf("found") // Usa un errore per interrompere la ricerca
			}
		}
		return nil
	})

	if err != nil && err.Error() != "found" {
		c.String(500, "Errore durante la ricerca: %v", err)
		return
	}

	c.String(404, "File non trovato con hash: %s", hashToFind)
}

func calculateSeekTime(videoPath, rangeHeader string) string {
	// Estrai byte offset dal Range header (es: "bytes=1048576-")
	var byteOffset int64
	if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-", &byteOffset); err != nil {
		return ""
	}

	// Se offset è troppo piccolo, inizia dall'inizio
	if byteOffset < 1024*1024 {
		return ""
	}

	// Ottieni durata totale del video
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	var duration float64
	if _, err := fmt.Sscanf(string(output), "%f", &duration); err != nil {
		return ""
	}

	// Ottieni dimensione file
	info, err := os.Stat(videoPath)
	if err != nil {
		return ""
	}
	fileSize := info.Size()

	// Calcola approssimativamente il tempo di seek
	// seekTime = (byteOffset / fileSize) * duration
	seekRatio := float64(byteOffset) / float64(fileSize)
	seekSeconds := seekRatio * duration

	// Ritorna in formato HH:MM:SS
	hours := int(seekSeconds) / 3600
	minutes := (int(seekSeconds) % 3600) / 60
	seconds := int(seekSeconds) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func streamTranscodedVideo(c *gin.Context, videoPath, seekTime string) {
	args := []string{}

	// Se c'è un seek time, aggiungi il parametro -ss
	if seekTime != "" {
		args = append(args, "-ss", seekTime)
	}

	args = append(args,
		"-i", videoPath,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "192k",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		"pipe:1",
	)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = c.Writer
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Errore ffmpeg: %v", err)
	}
}

func checkNeedsTranscode(path string) bool {
	// Controlla estensione - MKV non è supportato da Firefox
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mkv" || ext == ".avi" || ext == ".wmv" || ext == ".flv" {
		return true
	}

	// Usa ffprobe per leggere codec video
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	output, _ := cmd.Output()
	videoCodec := strings.TrimSpace(string(output))

	// Controlla anche il codec audio
	cmdAudio := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)

	audioOutput, _ := cmdAudio.Output()
	audioCodec := strings.TrimSpace(string(audioOutput))

	// H.264 + AAC sono nativamente supportati, il resto va transcodificato
	videoOk := videoCodec == "h264"
	audioOk := audioCodec == "aac" || audioCodec == "mp3"

	return !videoOk || !audioOk
}
