package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"assignment1/internal/server"
)

func main() {
	// Get port from command line --port
	// ./server -port xxxx
	// 0 means a random port will be assigned
	assignedPort := flag.String("port", "0", "Port to listen on")
	flag.Parse()

	// Create server instance
	srv, err := server.New(*assignedPort)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Channel to listen for OS signals (Ctrl+C, kill, etc.)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-stop

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop server gracefully
	if err := srv.Stop(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
