package dht

import "context"

type Transport interface {
	// Basic DHT RPCs
	CheckAlive(targetAddr string) (ok bool, err error)                        // RPC to check if the node at the given address is alive
	GetPredecessor(targetAddr string) (predecessor string, err error)         // RPC to get the predecessor of the node
	Notify(targetAddr string, predecessor string) error                       // RPC to notify the node at the given address that it might have a new predecessor
	FindSuccessor(targetAddr string, keyId int) (successor string, err error) // RPC to find the successor of the key
}

type INode interface {
	SetNetwork(network []string)
	SetTransport(transport Transport)
	RunMaintenance(ctx context.Context)
	Address() string
	Id() int
	Successor() (id int, address string)
	Predecessor() (id int, address string)
	SetPredecessor(predecessorAddr string)
	SetSuccessor(successorAddr string)
	FindSuccessor(keyId int) (successor string, err error)
	Get(key string) (value string, nextAddress string, err error)
	Put(key string, value string) (nextAddress string)
	String() string
	FingerTable() []string
}
