// Package store provides a generic key-value persistence layer. Domain
// packages (state, products, settings) wrap a Backend with their own typed
// API so the underlying storage can be swapped from files to a database
// without touching domain code.
package store

// Backend is the contract every storage implementation satisfies.
// Get returns (nil, nil) when the key is absent — not an error.
// Implementations must be safe for concurrent use.
type Backend interface {
	Get(key string) ([]byte, error)
	Put(key string, value []byte) error
	Close() error
}
