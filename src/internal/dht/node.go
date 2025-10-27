package dht

import (
	"fmt"
	"log"
	"sort"
	"sync"
)

const (
	M             = 16
	ID_SPACE_SIZE = 1 << M
)

type INode interface {
	Id() int
	Address() string
	Successor() (id int, address string)
	Predecessor() (id int, address string)
	Put(key string, value string) (nextNodeAddress string)
	Get(key string) (value string, nextNodeAddress string, err error)
	String() string
	FingerTable() []string
}

type Node struct {
	node
	predecessor node
	successor   node
	finger      []fingerEntry
	data        sync.Map
}

type node struct {
	id      int
	address string
}

type fingerEntry struct {
	start int
	node  node
}

// Create will create a new node and initialize the finger table from the whole network
// example address: "c11-1:50153"
func Create(address string, network []string) *Node {

	// self
	self := node{
		id:      KeyToRingId(address, ID_SPACE_SIZE),
		address: address,
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

	return &Node{
		node:        self,
		successor:   nodes[successorIdx],
		predecessor: nodes[predecessorIdx],
		finger:      finger,
	}
}

func (n *Node) Stabilize() {
	// Start the stabilize loop
	// TODO

	// Ask successor for its predecessor and decide if it should be our new successor
	// If so, update our successor to the new predecessor
	// If not, stabilized is reached.
	// TODO
}

func (n *Node) FixFingers() {
	// Fix the finger table of the node
	// TODO
}

func (n *Node) CheckPredecessor() {
	// Check if the predecessor is still alive
	// If not, update our predecessor to the successor of the predecessor
	// TODO
}

// Id returns the id of the node
func (n *Node) Id() int {
	return n.id
}

// Address returns the network address of the node
// Example: "c11-1:50153"
func (n *Node) Address() string {
	return n.address
}

// Successor
func (n *Node) Successor() (id int, address string) {
	return n.successor.id, n.successor.address
}

// Successor
func (n *Node) Predecessor() (id int, address string) {
	return n.predecessor.id, n.predecessor.address
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

		log.Printf("Node %d stored key '%s' (id: %d) and value length '%d'", n.id, key, keyId, len(value))
		return ""
	}

	// Lookup the finger table and return the closest preceeding node address
	_, closestPreceedingAddr := n.closestPreceedingNode(keyId)
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
				log.Printf("Node %d retrieved key '%s' (id: %d) and value length '%d'", n.id, key, keyId, len(strValue))
				return strValue, "", nil
			}
		}
		return "", "", fmt.Errorf("key not found")
	}
	_, closestPreceedingAddr := n.closestPreceedingNode(keyId)
	//log.Printf("Get(): Key '%s' (id: %d) not found, check address '%s' (id: %d)", key, keyId, closestPreceedingAddr, closestPreceedingId)

	// Lookup the finger table and return the closest preceeding node address
	return "", closestPreceedingAddr, nil
}

func (n *Node) closestPreceedingNode(keyId int) (id int, address string) {

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
