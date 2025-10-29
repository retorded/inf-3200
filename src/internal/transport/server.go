package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --------- NODE RPC HANDLERS ---------

// handleSuccessor handles GET/PUT requests to the "/successor" path
func (t *HTTPTransport) handleSuccessor(w http.ResponseWriter, r *http.Request) {

	// Switch on the method and perform Get/Set on node
	switch r.Method {
	case http.MethodGet:
		// GET: Read key from URL query parameter
		keyStr := r.URL.Query().Get("key")
		if keyStr == "" {
			http.Error(w, "key is required", http.StatusBadRequest)
			return
		}

		keyId, err := strconv.Atoi(keyStr)
		if err != nil {
			http.Error(w, "invalid key format", http.StatusBadRequest)
			return
		}

		successorAddr, err := t.node.FindSuccessor(keyId)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to find successor: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(successorAddr); err != nil {
			http.Error(w, "failed to encode successor", http.StatusInternalServerError)
			return
		}

	case http.MethodPut:
		// PUT: Read successor from JSON body
		var successor string
		if err := json.NewDecoder(r.Body).Decode(&successor); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		t.node.SetSuccessor(successor)

		// Send response confirming the update
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":    "success",
			"successor": successor,
		}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

// handlePredecessor handles requests to the "/predecessor" path
func (t *HTTPTransport) handlePredecessor(w http.ResponseWriter, r *http.Request) {

	// Switch on the method and perform Get/Set on node
	switch r.Method {
	case http.MethodGet:
		_, predecessorAddr := t.node.Predecessor()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(predecessorAddr); err != nil {
			http.Error(w, "failed to encode predecessor", http.StatusInternalServerError)
			return
		}

	case http.MethodPut:
		// Receive "Notify" request
		var predecessor string
		if err := json.NewDecoder(r.Body).Decode(&predecessor); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Suggest that the node might have a new predecessor
		t.node.Notify(predecessor)

		// Send response confirming the update
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(map[string]string{
			"status":      "success",
			"predecessor": predecessor,
		}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}

	case http.MethodPost:
		// Receive "SetPredecessor" request
		var predecessor string
		if err := json.NewDecoder(r.Body).Decode(&predecessor); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		// Instruct the node that has a new predecessor
		t.node.SetPredecessor(predecessor)
		w.WriteHeader(http.StatusOK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

// --------- SYSTEM HANDLERS ---------

// helloHandler handles requests to the "/ping" path
func (t *HTTPTransport) handlePing(w http.ResponseWriter, r *http.Request) {
	// Write the hostname and port to the response
	fmt.Fprintf(w, "%s", t.address)

	// Log the request
	//log.Printf("Ping request received from %s\n", r.RemoteAddr)
}

// handleStorage handles GET and PUT on the node.
// requests are forwarded if the node is not responsible for the key.
func (t *HTTPTransport) handleStorage(w http.ResponseWriter, r *http.Request) {

	log.Printf("handleStorage request received: %s, %s", r.URL.Path, r.Method)

	// Get the key from the request path
	key := strings.TrimPrefix(r.URL.Path, "/storage/")

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
		value, nextNodeAddress, err = t.node.Get(key)
		if err != nil {
			log.Printf("ERROR: Get failed for key %s: %v", key, err)
			http.NotFound(w, r)
			return
		}

	case http.MethodPut:
		nextNodeAddress = t.node.Put(key, string(body))

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Forward request if this node was not correct node
	if nextNodeAddress != "" {
		forwardURL := "http://" + nextNodeAddress + "/storage/" + key
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

// handleNetwork handles requests to the "/network" path
func (t *HTTPTransport) handleNetwork(w http.ResponseWriter, r *http.Request) {

	// PUT: NETWORK INITIALIZATION

	// Get the network from the request
	networkStr := r.URL.Query().Get("network")

	log.Printf("handleNetwork request received: %s, %s", r.URL.Query(), r.Method)

	if r.Method == http.MethodPut {

		// Parse network addresses
		var network []string
		if networkStr != "" {
			network = strings.Split(networkStr, ",")
			for i, addr := range network {
				network[i] = strings.TrimSpace(addr)
			}
		}
		// Create the ring node
		t.node.SetNetwork(network)
		log.Printf("Node updated with network: %s", t.node.String())
		w.WriteHeader(http.StatusOK)
		return
	}

	// GET: NETWORK TRAVERSAL

	// Recursive HTTP traversal approach.

	// Flag the origin of the traversal
	origin := r.URL.Query().Get("origin")
	if origin == "" {
		origin = t.node.Address() // first node
	}

	// Start list with this node
	nodes := []string{t.node.Address()}

	_, succAdr := t.node.Successor()

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
	if err := json.NewEncoder(w).Encode(nodes); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode nodes: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleNodeInfo handles requests to the "/node-info" path
func (t *HTTPTransport) handleNodeInfo(w http.ResponseWriter, r *http.Request) {

	type NodeInfo struct {
		NodeHash    string   `json:"node_hash"`
		Successor   string   `json:"successor"`
		Predecessor string   `json:"predecessor"`
		Others      []string `json:"others"`
	}

	nodeHash := strconv.Itoa(t.node.Id())
	_, successorAddress := t.node.Successor()
	_, predecessorAddress := t.node.Predecessor()
	others := t.node.FingerTable()

	info := NodeInfo{
		NodeHash:    nodeHash,
		Successor:   successorAddress,
		Predecessor: predecessorAddress,
		Others:      others,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode node info: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleJoin handles requests to the "/join" path
func (t *HTTPTransport) handleJoin(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get the nprime from the request
	nprime := r.URL.Query().Get("nprime")

	log.Printf("SERVER: Join request received, trying to join with nprime: %s", nprime)

	// Find the successor the loner node from nprime
	successorAddress, err := t.FindSuccessor(nprime, t.node.Id())
	if err != nil {
		log.Printf("ERROR: Failed to find successor this node %s: %v", nprime, err)
		http.Error(w, "failed to find successor", http.StatusInternalServerError)
		return
	}

	log.Printf("SERVER: Successor address found = %s", successorAddress)

	// Set the successor of the node. The maintenance goroutine will update the successor of the node.
	t.node.SetSuccessor(successorAddress)

	w.WriteHeader(http.StatusOK)
}

// handleLeave handles requests to the "/leave" path
func (t *HTTPTransport) handleLeave(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Make the node "plug" the hole
	err := t.node.Leave()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to leave: %v", err), http.StatusInternalServerError)
		return
	}

	// Set the node to inactive so it stops processing requests.
	// Other nodes will update their finger table and successor accordingly when they cannot reach the node.
	t.inactive = true

	log.Println("SERVER: Leave request received")
	w.WriteHeader(http.StatusOK)
}

// handleSimCrash handles requests to the "/sim-crash" path
func (t *HTTPTransport) handleSimCrash(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.inactive = true
	w.WriteHeader(http.StatusOK)
	log.Println("Sim crash request received")
}

// handleSimRecover handles requests to the "/sim-recover" path
func (t *HTTPTransport) handleSimRecover(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	t.inactive = false
	w.WriteHeader(http.StatusOK)
	log.Println("Sim recover request received")
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
		log.Printf("ERROR: Failed to create request for %s: %v", url, err)
		http.Error(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	if method == http.MethodPut {
		req.Header.Set("Content-Type", "text/plain")
	}

	// Add timeout to prevent hanging
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to forward %s to %s: %v", method, url, err)
		http.Error(w, fmt.Sprintf("failed to forward request: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	log.Printf("Forwarded %s to %s with status code %d", method, url, resp.StatusCode)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("ERROR: Failed to copy response from %s: %v", url, err)
	}
}

// refuseRequest returns a 503 Service Unavailable response
func refuseRequest(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "service unavailable", http.StatusServiceUnavailable)
}

// isServerError checks if the response indicates a server error (500/503)
func isServerError(resp *http.Response) bool {
	return resp.StatusCode == http.StatusInternalServerError || 
		   resp.StatusCode == http.StatusServiceUnavailable
}