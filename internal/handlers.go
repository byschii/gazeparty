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
	Name           string
	Path           string
	PathWithoutExt string
	IsDir          bool
	Hashed         string
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
		// Prova ad aggiungere estensioni video comuni
		resolvedPath := tryResolveVideoPath(requestedPath)
		if resolvedPath != "" {
			requestedPath = resolvedPath
			info, err = os.Stat(requestedPath)
		} else {
			c.String(404, "Non trovato")
			return
		}
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

		filePath := filepath.Join(requestedPath, file.Name())
		fileInfo := FileInfo{
			Name:           file.Name(),
			Path:           filePath,
			PathWithoutExt: removeVideoExtension(filePath),
			IsDir:          file.IsDir(),
			Hashed:         "",
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

	// Verifica che il file esista
	if _, err := os.Stat(videoPath); err != nil {
		// Prova a risolvere senza estensione
		resolvedPath := tryResolveVideoPath(videoPath)
		if resolvedPath != "" {
			videoPath = resolvedPath
		} else {
			c.String(404, "File non trovato: %s", videoPath)
			return
		}
	}

	// Nascondi l'estensione reale usando sempre video/mp4
	c.Header("Content-Type", "video/mp4")

	// Sempre transcodifica per compatibilità universale
	rangeHeader := c.GetHeader("Range")
	var seekTime string
	if rangeHeader != "" {
		seekTime = calculateSeekTime(videoPath, rangeHeader)
	}

	streamTranscodedVideo(c, videoPath, seekTime)
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

	// Seek veloce prima dell'input
	if seekTime != "" {
		args = append(args, "-ss", seekTime)
	}

	args = append(args, "-i", videoPath)

	// Sempre transcodifica per garantire compatibilità
	args = append(args,
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-crf", "28",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof+faststart",
		"-frag_duration", "1000000",
		"pipe:1",
	)

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = c.Writer
	cmd.Stderr = os.Stderr

	// Start the command
	if err := cmd.Start(); err != nil {
		log.Printf("Errore avvio ffmpeg: %v", err)
		return
	}

	// Monitor client disconnect
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-c.Request.Context().Done():
		// Client disconnected, kill ffmpeg
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		<-done // Wait for process to exit
	case err := <-done:
		// Process finished normally or with error
		if err != nil && !strings.Contains(err.Error(), "signal: killed") {
			log.Printf("Errore ffmpeg: %v", err)
		}
	}
}

// removeVideoExtension rimuove l'estensione video dal path
func removeVideoExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg"}

	for _, videoExt := range videoExts {
		if ext == videoExt {
			return strings.TrimSuffix(path, filepath.Ext(path))
		}
	}
	return path
}

// tryResolveVideoPath prova ad aggiungere estensioni video comuni per risolvere il path
func tryResolveVideoPath(path string) string {
	// Se esiste già, ritorna il path così com'è
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Prova con estensioni comuni
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg"}

	for _, ext := range videoExts {
		testPath := path + ext
		if _, err := os.Stat(testPath); err == nil {
			return testPath
		}
	}

	return ""
}
