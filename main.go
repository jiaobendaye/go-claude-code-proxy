package main

import (
	"log"
	"strconv"

	"github.com/jiaobendaye/go-claude-code-proxy/src/core"
	"github.com/jiaobendaye/go-claude-code-proxy/src/endpoints"
	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("Error loading .env file")
	}
}

func main() {
	config := core.GetConfig()
	config.Dump()
	router := endpoints.SetupOpenaiClientRouter()
	log.Printf("Starting server at %s:%d", config.Host, config.Port)
	if err := router.Run(config.Host + ":" + strconv.Itoa(config.Port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
