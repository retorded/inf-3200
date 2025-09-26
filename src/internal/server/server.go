package server

import (
	"assignment1/internal/dht"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
}

// New creates a new server instance with the specified port
func New(hostname string, port string, node dht.INode) (*Server, error) {

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Create server instance
	server := &Server{
		httpServer: httpServer,
		hostname:   hostname,
		port:       port,
		node:       node,
	}

	// Register handlers
	server.registerHandlers()

	log.Printf("Server running on hostname: %s, port: %s", hostname, port)
	log.Printf("Server node: %s", node.String())

	return server, nil
}

// GetHostName returns the hostname of the server
func (s *Server) GetHostName() string {
	return s.hostname
}

// GetPort returns the port of the server
func (s *Server) GetPort() string {
	return s.port
}

// registerHandlers sets up the HTTP route handlers
// TODO future: keep adding handlers here as needed. create separate handlers.go file.
func (s *Server) registerHandlers() {
	http.HandleFunc("/helloworld", s.helloHandler)
	http.HandleFunc("/storage/", s.handleStorage)
	http.HandleFunc("/network", s.handleNetwork)
}

// helloHandler handles requests to the "/helloworld" path
func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {

	key := r.URL.Path[len("/storage/"):]
	switch r.Method {
	case http.MethodGet:

		log.Printf("Server received storage GET for key '%s' ", key)

		// Get the value from the node
		value, nextNodeAddress, err := s.node.Get(key)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		if nextNodeAddress != "" {
			// Forward request to the next node
			redirectAddress := nextNodeAddress + "/storage/" + key
			http.Redirect(w, r, redirectAddress, http.StatusTemporaryRedirect)
			log.Printf("Server redirect GET to %s ", redirectAddress)
			return
		}

		// Write the response
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(value))
		if err != nil {
			http.Error(w, "failed to write response", http.StatusInternalServerError)
			return
		}

	case http.MethodPut:

		// Read the body of the request
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		log.Printf("Server received storage PUT for key: '%s', value: '%s'", key, string(body))

		// Put the key-value pair into the node
		nextNodeAddress := s.node.Put(key, string(body))

		// If the key doesn't belong to this node, forward the request
		if nextNodeAddress != "" {
			// Forward request to the next node
			http.Redirect(w, r, nextNodeAddress+"/storage/"+key, http.StatusTemporaryRedirect)
			return
		}

		// Write the response
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// helloHandler handles requests to the "/helloworld" path
func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO traverse the ring and get the network nodes
	nodes := []string{}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(nodes); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
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
