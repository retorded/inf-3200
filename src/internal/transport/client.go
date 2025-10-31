package transport

import (
	"assignment/internal/dht"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// HTTPTransport represents the HTTP transport with its configuration
type HTTPTransport struct {
	node       dht.INode
	server     *http.Server
	address    string
	inactive   bool
	fastClient *http.Client
	slowClient *http.Client
}

// New creates a new server instance
func New(hostname string, port string, node dht.INode) (*HTTPTransport, error) {

	mux := http.NewServeMux()

	t := &HTTPTransport{
		node:    node,
		address: hostname + ":" + port,
		slowClient: &http.Client{
			Timeout: 2 * time.Second,
		},
		fastClient: &http.Client{
			Timeout: 500 * time.Millisecond,
		},
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

	// node rpc endpoints
	mux.HandleFunc("/predecessor", t.handlePredecessor) // endpoint to get/put predecessor of the node
	mux.HandleFunc("/successor", t.handleSuccessor)     // endpoint to get/put the successor of the node

	// Wrap the mux with crash middleware
	t.server = &http.Server{
		Addr:    ":" + port,
		Handler: t.crashMiddleware(mux),
	}

	log.Printf("Transport created on '%s'", t.address)
	return t, nil
}

// crashMiddleware wraps the entire mux to check crash status
func (t *HTTPTransport) crashMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow sim-recover even when inactive
		if r.URL.Path == "/sim-recover" {
			next.ServeHTTP(w, r)
			return
		}

		// Refuse all other requests if inactive
		if t.inactive {
			refuseRequest(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
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

// =============== MAINTENACE RPC'S ===============

// FindSuccessor finds the successor of the key recursively
// Used in stabilization and join operations
func (t *HTTPTransport) FindSuccessor(addr string, keyId int) (successor string, err error) {

	keyIdStr := strconv.Itoa(keyId)

	// Use GET with query parameter
	resp, err := t.fastClient.Get("http://" + addr + "/successor?key=" + url.QueryEscape(keyIdStr))
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			err = fmt.Errorf("TIMEOUT: exceeded %v", ne.Timeout())
		}
		return "", err
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

// GetPredecessor gets the predecessor of the node
// Used in stabilization and leave operations
func (t *HTTPTransport) GetPredecessor(addr string) (string, error) {

	resp, err := t.fastClient.Get("http://" + addr + "/predecessor")
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			err = fmt.Errorf("TIMEOUT: exceeded %v", ne.Timeout())
		}
		return "", err
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
// Used in stabilization and join operations
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

	// Send request
	resp, err := t.fastClient.Do(req)
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

	resp, err := t.fastClient.Get("http://" + targetAddr + "/ping")
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			err = fmt.Errorf("TIMEOUT: exceeded %v", ne.Timeout())
		}
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// =============== AUTHORITY RPC'S ===============

// RPC's that are important to conclude the DHT operations, uses slow client to ensure reliability

// SetSuccessor sets the successor of the node
// 2 seconds timeout
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

	// Send request
	resp, err := t.slowClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to notify node at %s of new successor: %w", targetAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notify of new successor failed with status %d", resp.StatusCode)
	}

	return nil
}

// SetPredecessor sets the predecessor of the node
// 2 seconds timeout
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

	// Send request
	resp, err := t.slowClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to notify predecessor on %s: %w", targetAddr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notify predecessor failed with status %d", resp.StatusCode)
	}

	return nil

}

func (t *HTTPTransport) IsInactive() bool {
	return t.inactive
}
