// Package store wires together the blob and index backends and exposes a
// unified Store for writing profile samples.
package store

import (
	"context"
	"fmt"

	"profiler/internal/store/badgerblob"
	"profiler/internal/store/s3blob"
)

// Store composes a blob backend and an index backend.
type Store struct {
	blobs BlobStore
	index IndexStore
}

// IndexStore is the adapter interface for the profile metadata index.
// A concrete implementation (e.g. SQLite, Postgres) will be wired in here.
type IndexStore interface {
	RecordProfile(ctx context.Context, meta ProfileMeta) error
}

// ProfileSample is a raw profile payload received from an agent.
type ProfileSample struct {
	// Target is the scrape target identifier (e.g. pod name or service).
	Target string
	// Kind is the profile type (e.g. "cpu", "heap", "goroutine").
	Kind string
	// TimestampNano is the Unix nanosecond timestamp of the sample.
	TimestampNano int64
	// Data is the raw pprof (or other format) bytes.
	Data []byte
}

// Key returns the blob storage key for the sample.
// Format: "<target>/<kind>/<timestamp_ns>"
func (p ProfileSample) Key() string {
	return fmt.Sprintf("%s/%s/%d", p.Target, p.Kind, p.TimestampNano)
}

// ProfileMeta carries the index-level metadata for a stored sample.
type ProfileMeta struct {
	Key           string
	Target        string
	Kind          string
	TimestampNano int64
}

// Meta returns index-level metadata derived from the sample.
func (p ProfileSample) Meta(key string) ProfileMeta {
	return ProfileMeta{
		Key:           key,
		Target:        p.Target,
		Kind:          p.Kind,
		TimestampNano: p.TimestampNano,
	}
}

// NewBlobStore constructs the BlobStore described by cfg.
func NewBlobStore(ctx context.Context, cfg Config) (BlobStore, error) {
	switch cfg.BlobBackend {
	case "badger":
		return badgerblob.New(cfg.BadgerPath)
	case "s3":
		return s3blob.New(ctx, s3blob.Options{
			Endpoint:       cfg.S3Endpoint,
			Bucket:         cfg.S3Bucket,
			Region:         cfg.S3Region,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			ForcePathStyle: cfg.S3ForcePathStyle,
		})
	default:
		return nil, fmt.Errorf("store: unknown blob backend %q", cfg.BlobBackend)
	}
}

// New constructs a Store from the provided backends.
func New(blobs BlobStore, index IndexStore) *Store {
	return &Store{blobs: blobs, index: index}
}

// Write persists a profile sample: the raw bytes go to the blob store and
// the metadata goes to the index. If the index write fails the blob is
// left in place and will be collected by the orphan-sweep job.
func (s *Store) Write(ctx context.Context, sample ProfileSample) error {
	key := sample.Key()

	if err := s.blobs.Put(ctx, key, sample.Data); err != nil {
		return fmt.Errorf("store: writing blob: %w", err)
	}

	if err := s.index.RecordProfile(ctx, sample.Meta(key)); err != nil {
		// Blob is now orphaned but harmless — log for the sweep job.
		return fmt.Errorf("store: writing index: %w", err)
	}

	return nil
}
