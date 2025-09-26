package dht

import (
	"fmt"
	"log"
	"sort"
)

const (
	M             = 8
	ID_SPACE_SIZE = 1 << M
)

type INode interface {
	Id() int
	Address() string
	Put(key string, value string) (nextNodeAddress string)
	Get(key string) (value string, nextNodeAddress string, err error)
	String() string
}

type Node struct {
	node
	predecessor node
	successor   node
	finger      []fingerEntry
	data        map[string]string // Data stored in the node
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
	for i := range M {
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
		data:        make(map[string]string),
	}
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

// Put puts a key-value pair into the ring
func (n *Node) Put(key string, value string) (nextNodeAddress string) {

	// Hash the input key
	keyId := KeyToRingId(key, ID_SPACE_SIZE)

	// Check if the key is in the interval of the node
	if InInterval(keyId, n.id, n.successor.id) {
		n.data[key] = value
		log.Printf("Node %d stored key %s (id: %d) and value %s", n.id, key, keyId, value)
		return "" // Key stored locally, no forwarding needed
	}

	// Lookup the finger table and return the closest preceeding node address
	return n.closestPreceedingNode(keyId)
}

// Get gets a value from the ring
func (n *Node) Get(key string) (string, string, error) {

	// Hash the input key
	keyId := KeyToRingId(key, ID_SPACE_SIZE)

	// Check if the key is in the interval of the node
	if InInterval(keyId, n.id, n.successor.id) {
		if value, exists := n.data[key]; exists {
			log.Printf("Node %d retrieved key '%v' and value '%s'", n.id, key, value)
			return value, "", nil // Found locally, no forwarding needed
		}
		return "", "", fmt.Errorf("key not found")
	}

	// Lookup the finger table and return the closest preceeding node address
	return "", n.closestPreceedingNode(keyId), nil
}

func (n *Node) closestPreceedingNode(keyId int) string {
	// Iterate over the finger table and return the closest preceeding node address
	for i := len(n.finger) - 1; i >= 0; i-- {

		fingerId := n.finger[i].node.id

		// If the finger node is in the interval between this node and the key, return address of the node responsible for that interval
		if InInterval(fingerId, n.id, keyId) {
			return n.finger[i].node.address
		}
	}

	// If no finger table entry found, return successor
	log.Println("WARNING: No finger table entry found, returning successor")
	return n.successor.address
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
