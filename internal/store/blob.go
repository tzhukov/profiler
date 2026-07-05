package store

import "context"

// BlobStore is the adapter interface for raw profile blob storage.
// Implementations must be safe for concurrent use.
type BlobStore interface {
	// Put stores value under key, overwriting any existing entry.
	Put(ctx context.Context, key string, value []byte) error

	// Get retrieves the blob stored under key.
	// Returns ErrNotFound if the key does not exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes the blob stored under key.
	// A missing key is not an error.
	Delete(ctx context.Context, key string) error

	// Close releases any resources held by the store.
	Close() error
}
