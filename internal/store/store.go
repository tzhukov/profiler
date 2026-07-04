package store

import (
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type Store struct {
	blobs BlobStore
	index IndexStore
}

func NewBlobStore(cfg Config) (BlobStore, error) {
	switch cfg.BlobBackend {
	case "badger":
		return badger.New(cfg.BadgerPath)
	case "s3":
		return s3store.New(cfg.S3Endpoint, cfg.S3Bucket)
	default:
		return nil, fmt.Errorf("unknown blob backend: %s", cfg.BlobBackend)
	}
}

func (s *Store) Write(ctx context.Context, sample ProfileSample) error {
	key := sample.Key() // e.g. target/kind/timestamp
	if err := s.blobs.Put(ctx, key, sample.Data); err != nil {
		return fmt.Errorf("writing blob: %w", err)
	}
	if err := s.index.RecordProfile(ctx, sample.Meta(key)); err != nil {
		// blob is now orphaned but harmless — log for the sweep job
		return fmt.Errorf("writing index: %w", err)
	}
	return nil
}
