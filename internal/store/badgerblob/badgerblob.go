// Package badgerblob implements the store.BlobStore interface using
// Badger v4 as an embedded key-value store. It is intended as the PoC /
// single-node blob backend before S3 becomes available.
package badgerblob

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

// ErrNotFound is returned by Get when a key does not exist in the store.
var ErrNotFound = errors.New("badgerblob: key not found")

// Store wraps a Badger database and satisfies store.BlobStore.
type Store struct {
	db *badger.DB
}

// New opens (or creates) a Badger database at path and returns a Store.
// The caller is responsible for calling Close when finished.
func New(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("badgerblob: path must not be empty")
	}

	opts := badger.DefaultOptions(path).
		WithLogger(nil) // suppress Badger's internal log spam in production

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badgerblob: opening database at %q: %w", path, err)
	}

	return &Store{db: db}, nil
}

// Put stores value under key, replacing any existing entry.
func (s *Store) Put(_ context.Context, key string, value []byte) error {
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
	if err != nil {
		return fmt.Errorf("badgerblob: put %q: %w", key, err)
	}
	return nil
}

// Get retrieves the blob stored under key.
// Returns store.ErrNotFound when the key does not exist.
func (s *Store) Get(_ context.Context, key string) ([]byte, error) {
	var buf []byte

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		buf, err = item.ValueCopy(nil)
		return err
	})

	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("badgerblob: get %q: %w", key, err)
	}
	return buf, nil
}

// Delete removes the blob stored under key. A missing key is not an error.
func (s *Store) Delete(_ context.Context, key string) error {
	err := s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
	// Badger does not return an error for deleting a missing key, but be
	// defensive in case that ever changes.
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("badgerblob: delete %q: %w", key, err)
	}
	return nil
}

// Close flushes pending writes and releases all database resources.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("badgerblob: close: %w", err)
	}
	return nil
}
