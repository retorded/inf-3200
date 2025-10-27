package server

import (
	"assignment1/internal/dht"
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Server represents the HTTP server with its configuration
type Server struct {
	node       dht.INode
	httpServer *http.Server
	hostname   string
	port       string
	crash      bool
}

// New creates a new server instance
func New(hostname string, port string) (*Server, error) {

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Create server instance with the ring node
	server := &Server{
		httpServer: httpServer,
		hostname:   hostname,
		port:       port,
		node:       nil,
		crash:      false,
	}

	// Register handlers
	server.registerHandlers()

	log.Printf("Server created on %s:%s", hostname, port)

	return server, nil
}

// Start starts the HTTP server in a goroutine
func (s *Server) Start() error {
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

// GetHostName returns the hostname of the server
func (s *Server) GetHostName() string {
	return s.hostname
}

// GetPort returns the port of the server
func (s *Server) GetPort() string {
	return s.port
}
