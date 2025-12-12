package main

import (
	"log"

	"gazeparty/internal"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Carica i template quando necessario
	r.LoadHTMLGlob("templates/*")

	// Routing
	r.GET("/stream/*filepath", internal.HandleStream)
	r.GET("/find", internal.FindFromHash)

	// Usa NoRoute per catturare tutto il resto
	r.NoRoute(internal.HandleBrowse)

	log.Println("Server avviato su :8066")
	r.Run(":8066")
}
