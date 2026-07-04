package api

import (
	"context"
	"time"
)

type Target struct {
	Name      string
	Endpoint  string
	Namespace string
}

type ProfileSample struct {
	Target    Target
	Timestamp time.Time
	Kind      string //"cpu","heap","goroutine"
	Data      []byte
}

type Store interface {
	Write(ctx context.Context, sample ProfileSample) error
	Query(ctx context.Context, target string, kind string, from, to time.Time) ([]ProfileSample, error)
}
