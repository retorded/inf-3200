package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"assignment/internal/dht"
	"assignment/internal/transport"
)

func main() {

	// Get hostname from command line --hostname
	hostname := flag.String("hostname", "nil", "Hostname of the server")

	// Get assigned port from command line --port
	port := flag.String("port", "0", "Assigned port of the server")

	// Get log file directory
	logFilePath := flag.String("logfile", "", "Path to log file")
	flag.Parse()

	// Create or open log file
	file, err := os.OpenFile(*logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Set the log output to the file and log flags
	log.SetOutput(file)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Create node instance
	node := dht.Create(*hostname + ":" + *port)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	// Create HTTPTransport instance
	transport, err := transport.New(*hostname, *port, node)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server in goroutine
	go func() {
		if err := transport.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	log.Printf("Server started on '%s'", transport.Address())

	// Set transport so that node can use it to communicate with other nodes
	node.SetTransport(transport)

	// Start the maintenance goroutines
	go node.RunMaintenance(context.Background())

	// Channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Wait for shutdown signal
	<-stop

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Println("Server received shutdown signal")

	if err := transport.Stop(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
