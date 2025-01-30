package main

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/rohenaz/go-bmap-indexer/crawler"
	"github.com/rohenaz/go-bmap-indexer/state"
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
}

func main() {
	// Check if reset command is provided
	if len(os.Args) > 1 && os.Args[1] == "reset" {
		if len(os.Args) != 3 {
			log.Fatal("Usage: go run main.go reset <block_height>")
		}

		height, err := strconv.ParseUint(os.Args[2], 10, 32)
		if err != nil {
			log.Fatalf("Invalid block height: %v", err)
		}

		if err := state.ResetProgress(uint32(height)); err != nil {
			log.Fatalf("Failed to reset progress: %v", err)
		}

		log.Printf("Successfully reset indexer to block height %d", height)
		return
	}

	// Normal indexer operation
	currentBlock := state.LoadProgress()
	go crawler.ProcessDone()
	crawler.SyncBlocks(int(currentBlock))
	<-make(chan struct{})
}
