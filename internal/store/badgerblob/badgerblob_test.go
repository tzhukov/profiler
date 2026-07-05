package badgerblob_test

import (
	"context"
	"errors"
	"testing"

	"profiler/internal/store/badgerblob"
)

// newTestStore opens a Badger store in a temp directory and registers
// Close + directory removal via t.Cleanup, so the caller never needs
// to do either.
func newTestStore(t *testing.T) *badgerblob.Store {
	t.Helper()

	dir := t.TempDir() // automatically removed by t.Cleanup

	s, err := badgerblob.New(dir)
	if err != nil {
		t.Fatalf("badgerblob.New: %v", err)
	}

	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("store.Close: %v", err)
		}
	})

	return s
}

func TestPutAndGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	const key = "target/cpu/1234567890"
	data := []byte("pprof-payload")

	if err := s.Put(ctx, key, data); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Get = %q; want %q", got, data)
	}
}

func TestGetNotFound(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	_, err := s.Get(ctx, "does/not/exist")
	if !errors.Is(err, badgerblob.ErrNotFound) {
		t.Errorf("Get missing key: got %v; want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	const key = "target/heap/9999"

	if err := s.Put(ctx, key, []byte("data")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get(ctx, key)
	if !errors.Is(err, badgerblob.ErrNotFound) {
		t.Errorf("Get after Delete: got %v; want ErrNotFound", err)
	}
}

func TestDeleteMissingKeyIsNoError(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.Delete(ctx, "never/written"); err != nil {
		t.Errorf("Delete missing key: got %v; want nil", err)
	}
}

func TestPutOverwrites(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	const key = "target/goroutine/42"

	if err := s.Put(ctx, key, []byte("first")); err != nil {
		t.Fatalf("Put first: %v", err)
	}
	if err := s.Put(ctx, key, []byte("second")); err != nil {
		t.Fatalf("Put second: %v", err)
	}

	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("Get after overwrite = %q; want %q", got, "second")
	}
}
