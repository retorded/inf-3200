package transport

import (
	"assignment/internal/dht"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// HTTPTransport represents the HTTP transport with its configuration
type HTTPTransport struct {
	node    dht.INode
	server  *http.Server
	address string
	crash   bool
}

// New creates a new server instance
func New(hostname string, port string, node dht.INode) (*HTTPTransport, error) {

	mux := http.NewServeMux()

	t := &HTTPTransport{
		node: node,
		server: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
		address: hostname + ":" + port,
	}

	// system endpoints
	mux.HandleFunc("/ping", t.handlePing)
	mux.HandleFunc("/storage/", t.handleStorage)
	mux.HandleFunc("/network", t.handleNetwork)
	mux.HandleFunc("/node-info", t.handleNodeInfo)
	mux.HandleFunc("/join", t.handleJoin)
	mux.HandleFunc("/leave", t.handleLeave)
	mux.HandleFunc("/sim-crash", t.handleSimCrash)
	mux.HandleFunc("/sim-recover", t.handleSimRecover)

	// node endpoints
	mux.HandleFunc("/predecessor", t.handlePredecessor) // endpoint to get/put predecessor of the node
	mux.HandleFunc("/successor", t.handleSuccessor)     // endpoint to get the successor of the node

	log.Printf("Transport created on '%s'", t.address)

	return t, nil
}

// Start starts the HTTP server in a goroutine
func (t *HTTPTransport) Start() error {
	if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not start server: %w", err)
	}

	return nil
}

// Stop gracefully shuts down the server
func (t *HTTPTransport) Stop(ctx context.Context) error {
	log.Println("Shutting down server...")
	if err := t.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}
	log.Println("Server exited cleanly")
	return nil
}

func (t *HTTPTransport) Address() string {
	return t.address
}

// IMPLEMENTATION OF THE TRANSPORT INTERFACE
// RPC between nodes
func (t *HTTPTransport) GetPredecessor(addr string) (string, error) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get("http://" + addr + "/predecessor")
	if err != nil {
		return "", fmt.Errorf("failed to get predecessor from %s: %w", addr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("predecessor request failed with status %d", resp.StatusCode)
	}

	var predecessor string
	if err := json.NewDecoder(resp.Body).Decode(&predecessor); err != nil {
		return "", fmt.Errorf("failed to decode predecessor response: %w", err)
	}

	return predecessor, nil
}

// Notify notifies the node at the given address that it might have a new predecessor
func (t *HTTPTransport) Notify(targetAddr string, newPredecessor string) error {

	// Create JSON payload
	payload, err := json.Marshal(newPredecessor)
	if err != nil {
		return fmt.Errorf("failed to marshal predecessor: %w", err)
	}

	// Create PUT request
	req, err := http.NewRequest("PUT", "http://"+targetAddr+"/predecessor", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to notify predecessor on %s: %w", targetAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notify predecessor failed with status %d", resp.StatusCode)
	}

	return nil
}

// Notify notifies the node at the given address that it might have a new predecessor
func (t *HTTPTransport) SetSuccessor(targetAddr string, newSuccessor string) error {

	// Create JSON payload
	payload, err := json.Marshal(newSuccessor)
	if err != nil {
		return fmt.Errorf("failed to marshal successor: %w", err)
	}

	// Create PUT request
	req, err := http.NewRequest("PUT", "http://"+targetAddr+"/successor", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to notify node at %s of new successor: %w", targetAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notify of new successor failed with status %d", resp.StatusCode)
	}

	return nil
}

func (t *HTTPTransport) SetPredecessor(targetAddr string, newPredecessor string) error {

	// Create JSON payload
	payload, err := json.Marshal(newPredecessor)
	if err != nil {
		return fmt.Errorf("failed to marshal predecessor: %w", err)
	}

	// Create PUT request
	req, err := http.NewRequest("POST", "http://"+targetAddr+"/predecessor", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to notify predecessor on %s: %w", targetAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notify predecessor failed with status %d", resp.StatusCode)
	}

	return nil

}

// CheckAlive checks if the node at the given address is alive
func (t *HTTPTransport) CheckAlive(targetAddr string) (bool, error) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get("http://" + targetAddr + "/ping")
	if err != nil {
		return false, fmt.Errorf("failed to check if %s is alive: %w", targetAddr, err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// FindSuccessor finds the successor of the key recursively
func (t *HTTPTransport) FindSuccessor(addr string, keyId int) (successor string, err error) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	keyIdStr := strconv.Itoa(keyId)

	// Use GET with query parameter
	resp, err := client.Get("http://" + addr + "/successor?key=" + url.QueryEscape(keyIdStr))
	if err != nil {
		return "", fmt.Errorf("failed to find successor from %s: %w", addr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("find successor request failed with status %d", resp.StatusCode)
	}

	// Decode plain string response (not wrapped in object)
	if err := json.NewDecoder(resp.Body).Decode(&successor); err != nil {
		return "", fmt.Errorf("failed to decode successor response: %w", err)
	}

	return successor, nil
}
