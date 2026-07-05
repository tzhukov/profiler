package profile

import (
	"bytes"
	"fmt"

	"github.com/google/pprof/profile"
)

func Parse(data []byte) (*profile.Profile, error) {
	p, err := profile.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parsing profile: %w", err)
	}
	return p, nil
}

func Merge(profiles ...*profile.Profile) (*profile.Profile, error) {
	merged, err := profile.Merge(profiles)
	if err != nil {
		return nil, fmt.Errorf("merging profiles: %w", err)
	}
	return merged, nil
}
