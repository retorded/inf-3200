package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	hostname      string
	hostnameShort string
	assignedPort  *string
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Write the hostname and port to the response
	fmt.Fprintf(w, hostnameShort+":"+*assignedPort)

	// Log the request
	log.Printf("Request received from %s\n", r.RemoteAddr)
}

func main() {

	var err error

	// Hostname
	hostname, err = os.Hostname() // Gets local hostname
	if err != nil {
		log.Printf("Could not get hostname: %v", err)
	}
	// Extract just the short name
	hostnameShort = strings.Split(hostname, ".")[0]
	log.Printf("Server running on hostname: %s", hostnameShort)

	// Get port from command line --port
	// ./server -port xxxx
	// 0 means a random port will be assigned
	assignedPort = flag.String("port", "0", "Port to listen on")
	flag.Parse()

	// Register the handler function for the "/hello" path.
	// When a request comes to "/hello", helloHandler will be called.
	http.HandleFunc("/helloworld", helloHandler)

	// Create a new HTTP server
	server := &http.Server{
		Addr:         ":" + *assignedPort,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Channel to listen for OS signals (Ctrl+C, kill, etc.)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on port %s\n", *assignedPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-stop
	log.Println("Shutting down server...")

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly")
}
