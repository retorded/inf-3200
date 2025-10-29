package dht

import "context"

type Transport interface {
	// Basic DHT RPCs
	CheckAlive(targetAddr string) (ok bool, err error)                        // RPC to check if the node at the given address is alive
	GetPredecessor(targetAddr string) (predecessor string, err error)         // RPC to get the predecessor of the node
	Notify(targetAddr string, predecessor string) error                       // RPC to notify the node at the given address that it might have a new predecessor
	SetPredecessor(targetAddr string, predecessor string) error               // RPC to instruct the node at the given address that has a new predecessor
	SetSuccessor(targetAddr string, successor string) error                   // RPC to instruct the node at the given address that has a new successor
	FindSuccessor(targetAddr string, keyId int) (successor string, err error) // RPC to find the successor of the key

	// Crash handling
	IsCrashed() bool
}

type INode interface {
	SetNetwork(network []string)
	SetTransport(transport Transport)
	RunMaintenance(ctx context.Context)

	// Node setters and getters
	Address() string                       // Returns the network address of the node
	Id() int                               // Returns the id of the node
	Successor() (id int, address string)   // Returns the id and network address of the successor
	Predecessor() (id int, address string) // Returns the id and network address of the predecessor
	String() string                        // Returns a string representation of the node
	FingerTable() []string                 // Returns the finger table of the node

	// RPCs
	Notify(predecessor string)                                    // RPC to notify the node that it might have a new predecessor
	SetPredecessor(predecessor string)                            // RPC to instruct the node that has a new predecessor
	SetSuccessor(successor string)                                // RPC to instruct the node that has a new successor
	FindSuccessor(keyId int) (successor string, err error)        // RPC to find the successor of the key
	Get(key string) (value string, nextAddress string, err error) // RPC to get the value of the key
	Put(key string, value string) (nextAddress string)            // RPC to put the key-value pair into the ring
	Leave() error                                                 // RPC to leave the ring and return to starting state
}
