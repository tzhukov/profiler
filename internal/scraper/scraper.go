package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"profiler/internal/k8sclient"
	"profiler/internal/store"
	"time"
)

func scrape(ctx context.Context, store store.Store) {

	client, _ := k8sclient.New()

	pods, _ := client.PodsOnNode(ctx)

	// use async here
	for i, pod := range pods {
		profileBytes, error := fetchProfile(ctx, pod.Status.PodIP, 10)
		profileSample := store.ProfileSample{
			Target: pod.Name,
			// Kind is the profile type (e.g. "cpu", "heap", "goroutine").
			Kind: "test",
			// TimestampNano is the Unix nanosecond timestamp of the sample.
			TimestampNano: time.Now(),
			// Data is the raw pprof (or other format) bytes.
			Data: profileBytes,
		}
	}

	//use k8sclient to get node running on
	//use k8sclient to get pods running on the node we're running on
	//go through the list of pods and query each one for the profiler.

}

func fetchProfile(ctx context.Context, endpoint string, seconds int) ([]byte, error) {
	url := fmt.Sprintf("%s?seconds=%d", endpoint, seconds)

	client := &http.Client{
		// must exceed `seconds`, plus network/buffer slack
		Timeout: time.Duration(seconds+5) * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	return data, nil // already gzip-compressed pprof proto bytes
}
