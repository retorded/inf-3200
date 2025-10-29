package dht

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

const (
	M             = 16
	ID_SPACE_SIZE = 1 << M
)

type Node struct {
	node
	predecessor node
	successor   node
	finger      []fingerEntry
	data        sync.Map
	transport   Transport
	mu          sync.RWMutex
}

type node struct {
	id      int
	address string
}

type fingerEntry struct {
	start int
	node  node
}

func Create(address string) *Node {

	// self
	self := node{
		id:      KeyToRingId(address, ID_SPACE_SIZE),
		address: address,
	}

	finger := make([]fingerEntry, M)

	// Initialize finger table with self
	for i := 0; i < M; i++ {
		fingerKey := (self.id + (1 << i)) % ID_SPACE_SIZE
		finger[i] = fingerEntry{
			start: fingerKey,
			node:  self,
		}
	}

	node := &Node{
		node:        self,
		successor:   self,
		predecessor: node{},
		finger:      finger,
	}

	log.Printf("Node created with keyId: %d", node.Id())

	return node
}

// Set all node attributes based on the given network
func (n *Node) SetNetwork(network []string) {

	self := node{
		id:      n.Id(),
		address: n.Address(),
	}

	// Create all nodes and init with id and address for creation of finger table
	selfIndex := -1
	nodes := make([]node, 0, len(network))
	for _, nodeAddress := range network {
		nodes = append(nodes, node{
			id:      KeyToRingId(nodeAddress, ID_SPACE_SIZE),
			address: nodeAddress})
	}

	// Sort the nodes by id in ascending order
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].id < nodes[j].id
	})

	// Initialize finger table
	finger := make([]fingerEntry, M)
	for i := 0; i < M; i++ {
		fingerKey := (self.id + (1 << i)) % ID_SPACE_SIZE

		// Default: wrap around to first node
		chosen := nodes[0]

		// Find the first node with id >= fingerKey
		for _, node := range nodes {
			if node.id >= fingerKey {
				chosen = node
				break
			}
		}

		succ := node{
			id:      chosen.id,
			address: chosen.address,
		}

		finger[i] = fingerEntry{
			start: fingerKey,
			node:  succ,
		}
	}

	// Find this node's position in the sorted ring
	for i, node := range nodes {
		if node.id == self.id {
			selfIndex = i
			break
		}
	}

	if selfIndex == -1 {
		log.Println("WARNING: did not find self in network on init")
	}

	// Set successor and predecessor, indices account for wrap-around using modulo
	successorIdx := (selfIndex + 1) % len(nodes)
	predecessorIdx := (selfIndex - 1 + len(nodes)) % len(nodes)

	// Set successor, predecessor and finger table
	n.SetSuccessor(nodes[successorIdx].address)
	n.SetPredecessor(nodes[predecessorIdx].address)

	n.mu.Lock()
	defer n.mu.Unlock()
	n.finger = finger
}

// SetTransport sets the transport of the node
func (n *Node) SetTransport(transport Transport) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.transport = transport
}

// RunMaintenance runs the maintenance goroutines for the node at regular intervals.
func (n *Node) RunMaintenance(ctx context.Context) {
	stabilizeTicker := time.NewTicker(time.Duration(500) * time.Millisecond)
	fixFingerTicker := time.NewTicker(time.Duration(500) * time.Millisecond)
	checkPredTicker := time.NewTicker(time.Duration(1000) * time.Millisecond)

	defer func() {
		stabilizeTicker.Stop()
		fixFingerTicker.Stop()
		checkPredTicker.Stop()
	}()

	nextFingerIndex := 0

	for {
		select {
		case <-ctx.Done():
			return

		case <-stabilizeTicker.C:
			if !n.transport.IsCrashed() {
				go n.Stabilize()
			}

		case <-fixFingerTicker.C:
			if !n.transport.IsCrashed() {
				go n.FixFinger(nextFingerIndex)
				nextFingerIndex = (nextFingerIndex + 1) % M
			}

		case <-checkPredTicker.C:
			if !n.transport.IsCrashed() {
				go n.CheckPredecessor()
			}
		}
	}
}

// Stabilize stabilizes the node by updating the successor of the node
// Verify and update successor/predecessor links; detect node joins.
// New node runs stabilize which will inform others about its existence.
func (n *Node) Stabilize() {

	currSuccId, currSuccAddr := n.Successor()

	// If successor is self, we already know its predecessor locally
	if currSuccAddr == n.Address() {
		_, predAddr := n.Predecessor() // helper to return n.predecessor.address
		if predAddr != "" && predAddr != n.Address() {
			predId := KeyToRingId(predAddr, ID_SPACE_SIZE)
			if InIntervalOpen(predId, n.Id(), currSuccId) {
				n.SetSuccessor(predAddr)
			}
		}
	} else {
		// successor is another node — fetch its predecessor over transport
		candidates := n.closestSuccessorNodes()

		for _, candidate := range candidates {
			predAddr, err := n.transport.GetPredecessor(candidate)
			log.Printf("Stabilize: got predecessor '%s' from candidate '%s'", predAddr, candidate)
			if err != nil {
				log.Printf("Stabilize WARNING: failed to get predecessor from candidate successor '%s': %v", candidate, err)
				log.Println("Stabilize: candidates list", candidates)
				continue
			}

			if predAddr != "" && predAddr != n.Address() {
				predId := KeyToRingId(predAddr, ID_SPACE_SIZE)
				currSuccId, _ = n.Successor() // refresh
				if InIntervalOpen(predId, n.Id(), currSuccId) {
					n.SetSuccessor(predAddr)
					log.Printf("Stabilize: in interval, successor set to '%s' (id: '%d')", predAddr, predId)
					break // first live candidate
				}
			} else {
				// candidate alive but predecessor unknown, still use it
				n.SetSuccessor(candidate)
				log.Printf("Stabilize: not in interval, successor set to '%s' (id: '%d')", candidate, KeyToRingId(candidate, ID_SPACE_SIZE))
				break
			}
		}
	}

	// Update the "current successor" if new successor is set
	_, currSuccAddr = n.Successor()

	if currSuccAddr == n.Address() {
		// Skip notify if successor is self
		return
	}

	// Notify successor
	if err := n.transport.Notify(currSuccAddr, n.Address()); err != nil {
		log.Printf("Stabilize: failed to notify successor '%s': %v", currSuccAddr, err)
	}
}

func (n *Node) FixFinger(index int) {

	// Work directly with the original finger table
	n.mu.RLock()
	start := n.finger[index].start
	currentFingerAddr := n.finger[index].node.address
	n.mu.RUnlock()

	// Query the ring for the successor to the key
	successorAddr, err := n.FindSuccessor(start)
	if err != nil {
		log.Printf("Error finding successor for keyId '%v' from node '%s': %v", start, n.Address(), err)
		return
	}

	if successorAddr == n.Address() || successorAddr == currentFingerAddr {
		// Skip update if successor is self or current finger entry
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Update the finger
	successorId := KeyToRingId(successorAddr, ID_SPACE_SIZE)
	n.finger[index].node = node{
		id:      successorId,
		address: successorAddr,
	}

	log.Printf("FixFinger: entry at index %d set to '%s' (id: '%d')", index, successorAddr, successorId)

	if index == M-1 {
		log.Println("\n", n.String())
	}
}

// CheckPredecessor detects failed or disconnected predecessors.
func (n *Node) CheckPredecessor() {

	_, predAddr := n.Predecessor()

	// If the predecessor is the same as the node ("empty"), do nothing
	if predAddr == "" {
		// No predecessor found, do nothing (one-node ring)
		return
	}

	// Check if the predecessor is still alive
	alive, err := n.transport.CheckAlive(predAddr)
	if err != nil {
		log.Printf("Error checking if predecessor '%s' is alive: %v", predAddr, err)
		return
	}

	if !alive {
		// HANDLE FAILURE
		log.Printf("Predecessor '%s' is NOT alive, setting own predecessor to empty", predAddr)
		n.SetPredecessor("")
	}
}

func (n *Node) Leave() error {
	// Notify the successor that the node is leaving, and update the successor to the predecessor
	successorId, successorAddr := n.Successor()
	predecessorId, predecessorAddr := n.Predecessor()
	if err := n.transport.SetPredecessor(successorAddr, predecessorAddr); err != nil {
		return fmt.Errorf("failed to notify successor %s: %v", successorAddr, err)
	}
	// Notify the predecessor that its new successor is the successor of the node
	if err := n.transport.SetSuccessor(predecessorAddr, successorAddr); err != nil {
		return fmt.Errorf("failed to notify predecessor %s: %v", predecessorAddr, err)
	}

	log.Printf("Node '%s' (id: '%d') is leaving", n.Address(), n.Id())
	log.Printf("Closed the ring by connecting predecessor '%s' (id: '%d') to successor '%s' (id: '%d')", predecessorAddr, predecessorId, successorAddr, successorId)

	// Reset to starting state
	n.resetToStartingState()
	return nil
}

// Id returns the id of the node
func (n *Node) Id() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.id
}

// Address returns the network address of the node
// Example: "c11-1:50153"
func (n *Node) Address() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.address
}

// Successor returns the succesor node
func (n *Node) Successor() (id int, address string) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.successor.id, n.successor.address
}

// Successor
func (n *Node) Predecessor() (id int, address string) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.predecessor.id, n.predecessor.address
}

// Notify notifies the node that it might have a new predecessor
func (n *Node) Notify(suggestedPredecessorAddr string) {

	_, currentPredecessorAddr := n.Predecessor()

	if suggestedPredecessorAddr == "" || suggestedPredecessorAddr == n.Address() {
		return
	}

	suggestedPredecessorId := KeyToRingId(suggestedPredecessorAddr, ID_SPACE_SIZE)

	// Accept if predecessor is empty OR in (predecessor, self]
	if currentPredecessorAddr == "" || InIntervalRightInclusive(suggestedPredecessorId, n.predecessor.id, n.id) {
		n.SetPredecessor(suggestedPredecessorAddr)
	}
}

// SetPredecessor updates the predecessor if the suggested node is closer.
func (n *Node) SetPredecessor(predecessorAddr string) {

	n.mu.Lock()
	defer n.mu.Unlock()

	if predecessorAddr == "" {
		n.predecessor = node{}
		log.Printf("Set own predecessor to empty")
		return
	}

	potentialPredecessorId := KeyToRingId(predecessorAddr, ID_SPACE_SIZE)

	// Accept if predecessor is empty OR not the same as the node
	if n.predecessor.address == "" || potentialPredecessorId != n.id {

		n.predecessor = node{
			id:      potentialPredecessorId,
			address: predecessorAddr,
		}
		log.Printf("SetPredecessor to '%s' (id: '%d')", n.predecessor.address, n.predecessor.id)
	}
}

// SetSuccessor sets the successor of the node
func (n *Node) SetSuccessor(successorAddr string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.successor = node{
		id:      KeyToRingId(successorAddr, ID_SPACE_SIZE),
		address: successorAddr,
	}
	log.Printf("SetSuccessor to '%s' (id: '%d')", n.successor.address, n.successor.id)
}

// Put puts a key-value pair into the ring
func (n *Node) Put(key string, value string) (nextNodeAddress string) {

	// Hash the input key
	keyId := KeyToRingId(key, ID_SPACE_SIZE)

	// Each key is stored in the successor of key
	// Successor of k = the first node whose ID is greater than or equal to k
	if InIntervalRightInclusive(keyId, n.predecessor.id, n.id) {
		// Thread-safe store using sync.Map
		n.data.Store(key, value)

		log.Printf("Node '%d' stored key '%s' (id: '%d') and value length '%d'", n.id, key, keyId, len(value))
		return ""
	}

	// Lookup the finger table and return the closest preceeding node address
	_, closestPreceedingAddr := n.closestPrecedingNode(keyId)
	//log.Printf("Put(): Key '%s' (id: %d) not found, check address '%s' (id: %d)", key, keyId, closestPreceedingAddr, closestPreceedingId)

	// Lookup the finger table and return the closest preceeding node address
	return closestPreceedingAddr
}

// Get gets a value from the ring
func (n *Node) Get(key string) (value string, nextAddress string, err error) {

	// Hash the input key
	keyId := KeyToRingId(key, ID_SPACE_SIZE)

	// Check if the key is in the interval from the preceeding to self
	// If the key id == node id, this node takes ownership
	if InIntervalRightInclusive(keyId, n.predecessor.id, n.id) {

		// Thread-safe load
		if value, exists := n.data.Load(key); exists {
			if strValue, ok := value.(string); ok {
				log.Printf("Node '%d' retrieved key '%s' (id: '%d') and value length '%d'", n.id, key, keyId, len(strValue))
				return strValue, "", nil
			}
		}
		return "", "", fmt.Errorf("key not found")
	}
	_, closestPreceedingAddr := n.closestPrecedingNode(keyId)
	//log.Printf("Get(): Key '%s' (id: %d) not found, check address '%s' (id: %d)", key, keyId, closestPreceedingAddr, closestPreceedingId)

	// Lookup the finger table and return the closest preceeding node address
	return "", closestPreceedingAddr, nil
}

// FindSuccessor finds the successor of the input
func (n *Node) FindSuccessor(keyId int) (successor string, err error) {

	successorId, successorAddr := n.Successor()

	// Check if this node's successor is the successor for the key
	if InIntervalRightInclusive(keyId, n.Id(), successorId) {
		return successorAddr, nil
	}

	// Search the finger table for the closest preceeding nodes
	candidates := n.closestPrecedingNodes(keyId)
	for _, candidate := range candidates {
		successor, err = n.transport.FindSuccessor(candidate, keyId)
		if err != nil {
			log.Printf("Error finding successor for keyId '%v' from node '%s': %v", keyId, candidate, err)
			continue
		}
		return successor, nil
	}

	log.Printf("FindSuccessor: all candidates failed, falling back to successor %s", successorAddr)
	return successorAddr, nil

}

func (n *Node) closestPrecedingNodes(keyId int) (candidates []string) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for i := len(n.finger) - 1; i >= 0; i-- {
		fingerId := n.finger[i].node.id
		if InIntervalOpen(fingerId, n.id, keyId) {
			candidates = append(candidates, n.finger[i].node.address)
		}
	}
	return candidates
}

func (n *Node) closestSuccessorNodes() (candidates []string) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	seen := make(map[string]bool)

	// Include immediate successor first
	if n.successor.address != n.address {
		candidates = append(candidates, n.successor.address)
		seen[n.successor.address] = true
	}

	for _, finger := range n.finger {
		addr := finger.node.address
		if addr != n.address && !seen[addr] {
			candidates = append(candidates, addr)
			seen[addr] = true
		}
	}

	return candidates
}

func (n *Node) closestPrecedingNode(keyId int) (id int, address string) {

	// Iterate over the finger table and return the closest preceeding node address
	for i := len(n.finger) - 1; i >= 0; i-- {

		fingerId := n.finger[i].node.id

		// Open interval to fulfull the strict closest "preceeding"
		if InIntervalOpen(fingerId, n.id, keyId) {
			return fingerId, n.finger[i].node.address
		}
	}

	// If no finger table entry found, return successor
	return n.successor.id, n.successor.address
}

// String returns a string representation of the node
func (n *Node) String() string {
	out := fmt.Sprintf("ID: %d, Address: '%s'\n", n.id, n.address)
	out += fmt.Sprintf("  Successor: %d ('%s')\n", n.successor.id, n.successor.address)
	out += fmt.Sprintf("  Predecessor: %d ('%s')\n", n.predecessor.id, n.predecessor.address)
	out += "  Finger table:\n"
	for i, f := range n.finger {
		out += fmt.Sprintf("    [%d] start=%d --> successor=%d ('%s')\n",
			i, f.start, f.node.id, f.node.address)
	}
	return out
}

func (n *Node) FingerTable() []string {
	addresses := make([]string, M)
	for i, f := range n.finger {
		addresses[i] = f.node.address
	}
	return addresses
}

// HELPER

func (n *Node) resetToStartingState() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.successor = node{
		id:      n.id,
		address: n.address,
	}
	n.predecessor = node{}

	finger := make([]fingerEntry, M)

	// Initialize finger table with self
	for i := 0; i < M; i++ {
		fingerKey := (n.id + (1 << i)) % ID_SPACE_SIZE
		finger[i] = fingerEntry{
			start: fingerKey,
			node:  n.node,
		}
	}
}
