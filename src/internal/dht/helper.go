package dht

import (
	"crypto/sha1"
	"math/big"
)

// KeyToRingId hashes the input string and returns an int in the range 0..(mod-1)
// mod is the number of nodes in the ring (id space size)
func KeyToRingId(key string, mod int) int {

	// Compute SHA-1 hash of key
	h := sha1.New()
	h.Write([]byte(key))
	hashBytes := h.Sum(nil)

	// Convert hash bytes to a big integer
	hashInt := new(big.Int).SetBytes(hashBytes)

	// Map hash to ring using modulo M
	modInt := new(big.Int).Mod(hashInt, big.NewInt(int64(mod)))

	return int(modInt.Int64())
}

// check if x is in (a, b] on the ring
func InInterval(x int, a int, b int) bool {
	if a < b {
		return x > a && x <= b
	} else {
		// Wrap-around case
		return x > a || x <= b
	}
}