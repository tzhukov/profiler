// Package scraper periodically fetches pprof profiles from every pod running
// on this agent's node and writes them to the store.
//
// Lifecycle:
//
//	s := scraper.New(cfg, k8sClient, store)
//	s.Run(ctx)  // blocks until ctx is cancelled
package scraper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	"profiler/internal/store"
)

// PodLister is the subset of k8sclient.Client the scraper needs.
// Using an interface keeps the scraper testable without a real cluster.
type PodLister interface {
	PodsOnNode(ctx context.Context) ([]corev1.Pod, error)
}

// Writer is the subset of store.Store the scraper needs.
type Writer interface {
	Write(ctx context.Context, sample store.ProfileSample) error
}

// Config holds the tunables for the scrape loop.
type Config struct {
	// Interval is how often a full scrape round is triggered.
	// Defaults to 60s if zero.
	Interval time.Duration

	// ProfileSeconds is the pprof collection window passed to each target.
	// Defaults to 10s if zero.
	ProfileSeconds int

	// ProfileKind is the profile type label stored with each sample
	// (e.g. "cpu", "heap"). Defaults to "cpu" if empty.
	ProfileKind string
}

func (c *Config) withDefaults() Config {
	out := *c
	if out.Interval <= 0 {
		out.Interval = 60 * time.Second
	}
	if out.ProfileSeconds <= 0 {
		out.ProfileSeconds = 10
	}
	if out.ProfileKind == "" {
		out.ProfileKind = "cpu"
	}
	return out
}

// Scraper drives the periodic scrape loop.
type Scraper struct {
	cfg    Config
	pods   PodLister
	store  Writer
	logger *slog.Logger
}

// New constructs a Scraper. logger may be nil, in which case the default
// slog logger is used.
func New(cfg Config, pods PodLister, store Writer, logger *slog.Logger) *Scraper {
	if logger == nil {
		logger = slog.Default()
	}
	return &Scraper{
		cfg:    cfg.withDefaults(),
		pods:   pods,
		store:  store,
		logger: logger,
	}
}

// Run blocks, executing a scrape round on every tick, until ctx is cancelled.
// It returns only after the in-flight round (if any) has completed, so the
// caller can safely shut down the store immediately after Run returns.
func (s *Scraper) Run(ctx context.Context) {
	s.logger.Info("scraper starting", "interval", s.cfg.Interval)

	// Fire immediately on start, then on every tick.
	s.round(ctx)

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scraper stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			s.round(ctx)
		}
	}
}

// round performs one full scrape of all pods on this node. Each pod is
// scraped concurrently; errors are logged and skipped, never fatal.
func (s *Scraper) round(ctx context.Context) {
	pods, err := s.pods.PodsOnNode(ctx)
	if err != nil {
		s.logger.Error("listing pods", "err", err)
		return
	}

	s.logger.Info("scrape round started", "pods", len(pods))

	var wg sync.WaitGroup
	for _, pod := range pods {
		if !podScrapeable(pod) {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.scrapePod(ctx, pod)
		}()
	}
	wg.Wait()

	s.logger.Info("scrape round complete")
}

// scrapePod fetches a profile from a single pod and writes it to the store.
func (s *Scraper) scrapePod(ctx context.Context, pod corev1.Pod) {
	log := s.logger.With("pod", pod.Name, "namespace", pod.Namespace)

	data, err := fetchProfile(ctx, pod.Status.PodIP, s.cfg.ProfileSeconds)
	if err != nil {
		log.Warn("fetch failed", "err", err)
		return
	}

	sample := store.ProfileSample{
		Target:        fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
		Kind:          s.cfg.ProfileKind,
		TimestampNano: time.Now().UnixNano(),
		Data:          data,
	}

	if err := s.store.Write(ctx, sample); err != nil {
		log.Error("store write failed", "err", err)
		return
	}

	log.Info("profile stored", "bytes", len(data))
}

// podScrapeable returns true if the pod is running and has an IP assigned.
func podScrapeable(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning && pod.Status.PodIP != ""
}

// fetchProfile performs a blocking pprof HTTP fetch against the pod's IP.
func fetchProfile(ctx context.Context, podIP string, seconds int) ([]byte, error) {
	url := fmt.Sprintf("http://%s/debug/pprof/profile?seconds=%d", podIP, seconds)

	httpClient := &http.Client{
		// Total timeout must exceed the collection window plus slack.
		Timeout: time.Duration(seconds+5) * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, podIP)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	return data, nil
}
