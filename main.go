package main

import (
	"gazeparty/internal"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Load video data from file and sync with media directory
	if _, err := internal.LoadAndSyncVideos(); err != nil {
		panic(err)
	}

	// Start background cleanup: check every 1min, remove files older than 8min
	internal.StartCleanup(1*time.Minute, 8*time.Minute)

	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})
	r.GET("/player", func(c *gin.Context) {
		c.File("./static/player.html")
	})
	r.Static("/static", "./static")
	r.GET("/files", internal.HandleFiles)
	r.GET("/stream/:id/playlist.m3u8", internal.HandlePlaylist)
	r.GET("/stream/:id/:n", internal.HandleSegment)

	r.Run(":8066")
}
