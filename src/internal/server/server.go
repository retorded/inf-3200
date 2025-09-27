package server

import (
	"assignment1/internal/dht"
	"bytes"
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

// handleStorage handles GET and PUT on the node.
// requests are forwarded if the node is not responsible for the key.
func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {

	// Get the key
    key := r.URL.Path[len("/storage/"):]

	// Extract body of PUT
    var body []byte
    if r.Method == http.MethodPut {
        var err error
        body, err = io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "failed to read body", http.StatusInternalServerError)
            return
        }
    }

    var nextNodeAddress string
    var value string
    var err error

	// Switch on the method and perform Get/Put on node
    switch r.Method {
    case http.MethodGet:
        value, nextNodeAddress, err = s.node.Get(key)
        if err != nil {
            http.NotFound(w, r)
            return
        }

    case http.MethodPut:
        nextNodeAddress = s.node.Put(key, string(body))

    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

	// Forward request if this node was not correct node
    if nextNodeAddress != "" {
        forwardURL := "http://" + nextNodeAddress + "/storage/" + key
        log.Printf("Forwarding %s to %s", r.Method, forwardURL)
        forwardRequest(w, r.Method, forwardURL, body)
        return
    }

	// Write value to body if GET, otherwise write header status OK for PUT
    if r.Method == http.MethodGet {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(value))
    } else {
        w.WriteHeader(http.StatusOK)
    }
}

// helloHandler handles requests to the "/helloworld" path
func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Recursive HTTP traversal approach.

	// Flag the origin of the traversal
	origin := r.URL.Query().Get("origin")
	if origin == "" {
		origin = s.node.Address() // first node
	}

	// Start list with this node
	nodes := []string{s.node.Address()}

	_, succAdr := s.node.Successor()

	// We keep forwarding request, add node to list if not the origin.
	if succAdr != origin {
		forwardURL := fmt.Sprintf("http://%s/network?origin=%s", succAdr, origin)
		resp, err := http.Get(forwardURL)
		if err == nil {
			defer resp.Body.Close()
			var succNodes []string
			if err := json.NewDecoder(resp.Body).Decode(&succNodes); err == nil {
				nodes = append(nodes, succNodes...)
			}
		} else {
			log.Printf("Failed to contact successor %s: %v", succAdr, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
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

// HELPER

func forwardRequest(w http.ResponseWriter, method, url string, body []byte) {
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	if method == http.MethodPut {
		req.Header.Set("Content-Type", "text/plain")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to forward request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response: %v", err)
	}
}
