package store

import "errors"

// ErrNotFound is returned by BlobStore.Get when a key does not exist.
var ErrNotFound = errors.New("store: key not found")
