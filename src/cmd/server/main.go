package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"assignment1/internal/dht"
	"assignment1/internal/server"
)

func main() {

	// Get hostname, extract just the short name
	hostname, _ := os.Hostname()
	hostnameShort := strings.Split(hostname, ".")[0]

	// Get port from command line --port
	// ./server -port xxxx
	// 0 means a random port will be assigned
	assignedPort := flag.String("port", "0", "Port to listen on")

	// Get the network nodes (comma-separated list of addresses with ports)
	networkStr := flag.String("network", "", "Comma-separated list of network node addresses (e.g., c11-1:50153,c11-2:50154)")
	flag.Parse()

	// Parse network addresses
	var network []string
	if *networkStr != "" {
		network = strings.Split(*networkStr, ",")
		// Trim whitespace from each address
		for i, addr := range network {
			network[i] = strings.TrimSpace(addr)
		}
	}

	// Create the ring node, initialize with full network
	node := dht.Create(hostnameShort+":"+*assignedPort, network)

	// Create server instance
	srv, err := server.New(hostnameShort, *assignedPort, node)
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