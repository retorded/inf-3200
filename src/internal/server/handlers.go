package server

import (
	"assignment1/internal/dht"
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

func (s *Server) registerHandlers() {
	http.HandleFunc("/helloworld", s.helloHandler)
	http.HandleFunc("/storage/", s.handleStorage)
	http.HandleFunc("/network", s.handleNetwork)
	http.HandleFunc("/node-info", s.handleNodeInfo)
	http.HandleFunc("/join", s.handleJoin)
	http.HandleFunc("/leave", s.handleLeave)
	http.HandleFunc("/sim-crash", s.handleSimCrash)
	http.HandleFunc("/sim-recover", s.handleSimRecover)
}

// handleStorage handles GET and PUT on the node.
// requests are forwarded if the node is not responsible for the key.
func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request) {

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
		value, nextNodeAddress, err = s.node.Get(key)
		if err != nil {
			log.Printf("ERROR: Get failed for key %s: %v", key, err)
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
		s.node = dht.Create(s.hostname+":"+s.port, network)
		log.Printf("Node created: %s", s.node.String())
		w.WriteHeader(http.StatusOK)
		return
	}

	// GET: NETWORK TRAVERSAL

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
	if err := json.NewEncoder(w).Encode(nodes); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode nodes: %v", err), http.StatusInternalServerError)
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

// handleNodeInfo handles requests to the "/node-info" path
func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {

	type NodeInfo struct {
		NodeHash  string   `json:"node_hash"`
		Successor string   `json:"successor"`
		Others    []string `json:"others"`
	}

	nodeHash := strconv.Itoa(s.node.Id())
	_, successorAddress := s.node.Successor()
	others := s.node.FingerTable()

	info := NodeInfo{
		NodeHash:  nodeHash,
		Successor: successorAddress,
		Others:    others,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(info); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode node info: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleJoin handles requests to the "/join" path
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	log.Println("Join request received")
}

// handleLeave handles requests to the "/leave" path
func (s *Server) handleLeave(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	log.Println("Leave request received")
}

// handleSimCrash handles requests to the "/sim-crash" path
func (s *Server) handleSimCrash(w http.ResponseWriter, r *http.Request) {
	s.crash = true
	w.WriteHeader(http.StatusOK)
	log.Println("Sim crash request received")
}

// handleSimRecover handles requests to the "/sim-recover" path
func (s *Server) handleSimRecover(w http.ResponseWriter, r *http.Request) {
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
