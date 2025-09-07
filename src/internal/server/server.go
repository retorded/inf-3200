package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Server represents the HTTP server with its configuration
type Server struct {
	httpServer *http.Server
	hostname   string
	port       string
}

// New creates a new server instance with the specified port
func New(port string) (*Server, error) {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("could not get hostname: %w", err)
	}

	// Extract just the short name
	hostnameShort := strings.Split(hostname, ".")[0]
	log.Printf("Server running on hostname: %s", hostnameShort)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Create server instance
	server := &Server{
		httpServer: httpServer,
		hostname:   hostnameShort,
		port:       port,
	}

	// Register handlers
	server.registerHandlers()

	return server, nil
}

// registerHandlers sets up the HTTP route handlers
// TODO future: keep adding handlers here as needed. create separate handlers.go file.
func (s *Server) registerHandlers() {
	// Register the handler function for the "/helloworld" path
	http.HandleFunc("/helloworld", s.helloHandler)
}

// helloHandler handles requests to the "/helloworld" path
func (s *Server) helloHandler(w http.ResponseWriter, r *http.Request) {
	// Write the hostname and port to the response
	fmt.Fprintf(w, "%s:%s", s.hostname, s.port)

	// Log the request
	log.Printf("Request received from %s\n", r.RemoteAddr)
}

// Start starts the HTTP server in a goroutine
func (s *Server) Start() error {
	log.Printf("Server starting on port %s\n", s.port)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not start server: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the server
func (s *Server) Stop(ctx context.Context) error {
	log.Println("Shutting down server...")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}
	log.Println("Server exited cleanly")
	return nil
}
