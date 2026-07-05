// Command agent is the profiler DaemonSet process. It runs on every node,
// discovers pods via the Kubernetes API, and periodically scrapes pprof
// profiles from them, writing results to the configured blob store.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"profiler/internal/k8sclient"
	"profiler/internal/scraper"
	"profiler/internal/store"
	"profiler/internal/store/badgerblob"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("agent exiting with error", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// --- context: cancelled on SIGTERM / SIGINT ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	// --- blob store ---
	blobPath := envOr("BLOB_PATH", "/var/lib/profiler/blobs")
	blobs, err := badgerblob.New(blobPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := blobs.Close(); err != nil {
			logger.Error("closing blob store", "err", err)
		}
	}()

	// index is not wired yet; use a no-op until the SQLite adapter lands.
	st := store.New(blobs, noopIndex{})

	// --- k8s client ---
	k8s, err := k8sclient.New()
	if err != nil {
		return err
	}

	// --- scraper ---
	interval := envDuration("SCRAPE_INTERVAL", 60*time.Second)
	sc := scraper.New(
		scraper.Config{
			Interval:       interval,
			ProfileSeconds: 10,
			ProfileKind:    "cpu",
		},
		k8s,
		st,
		logger,
	)

	sc.Run(ctx) // blocks until signal
	return nil
}

// noopIndex satisfies store.IndexStore until a real implementation is wired in.
type noopIndex struct{}

func (noopIndex) RecordProfile(_ context.Context, _ store.ProfileMeta) error { return nil }

// envOr returns the value of the named environment variable, or fallback if unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envDuration returns a duration from an environment variable, or fallback if
// the variable is unset or unparseable.
func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid duration env var, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return d
}
