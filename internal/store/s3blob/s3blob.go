// Package s3blob implements the store.BlobStore interface using an S3-compatible
// object store (AWS S3 or MinIO). It is the v1 blob backend.
//
// Credentials are resolved in order:
//  1. Explicit S3AccessKey / S3SecretKey from config (static credentials)
//  2. AWS default credential chain (env vars, ~/.aws/credentials, IMDS, …)
package s3blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// ErrNotFound is returned by Get when the requested object does not exist.
var ErrNotFound = errors.New("s3blob: key not found")

// Store wraps an S3 client and satisfies store.BlobStore.
type Store struct {
	client *s3.Client
	bucket string
}

// Options configures the S3 client.
type Options struct {
	// Endpoint overrides the default AWS endpoint (e.g. "http://minio:9000").
	// Leave blank to use standard AWS endpoints.
	Endpoint string

	// Bucket is the S3 bucket name. Required.
	Bucket string

	// Region is the AWS region (e.g. "us-east-1").
	// Optional for MinIO; required for AWS S3.
	Region string

	// AccessKey / SecretKey are static credentials.
	// Leave blank to use the default AWS credential chain.
	AccessKey string
	SecretKey string

	// ForcePathStyle uses path-style addressing, which is required for MinIO.
	ForcePathStyle bool
}

// New constructs a Store from the given Options and verifies that the bucket
// is reachable by performing a HeadBucket call.
func New(ctx context.Context, opts Options) (*Store, error) {
	if opts.Bucket == "" {
		return nil, fmt.Errorf("s3blob: bucket name must not be empty")
	}

	cfgOpts := []func(*awsconfig.LoadOptions) error{}

	if opts.Region != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithRegion(opts.Region))
	}

	if opts.AccessKey != "" && opts.SecretKey != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, ""),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return nil, fmt.Errorf("s3blob: loading AWS config: %w", err)
	}

	s3Opts := []func(*s3.Options){}

	if opts.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(opts.Endpoint)
		})
	}

	if opts.ForcePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(cfg, s3Opts...)

	// Validate connectivity and bucket existence at construction time.
	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(opts.Bucket),
	}); err != nil {
		return nil, fmt.Errorf("s3blob: bucket %q not accessible: %w", opts.Bucket, err)
	}

	return &Store{client: client, bucket: opts.Bucket}, nil
}

// Put uploads value as an S3 object under key, replacing any existing object.
func (s *Store) Put(ctx context.Context, key string, value []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(value),
		ContentLength: aws.Int64(int64(len(value))),
	})
	if err != nil {
		return fmt.Errorf("s3blob: put %q: %w", key, err)
	}
	return nil
}

// Get downloads and returns the object stored under key.
// Returns store.ErrNotFound when the object does not exist.
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("s3blob: get %q: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("s3blob: reading body for %q: %w", key, err)
	}
	return data, nil
}

// Delete removes the object stored under key. A missing key is not an error.
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("s3blob: delete %q: %w", key, err)
	}
	return nil
}

// Close is a no-op for the S3 backend (the HTTP client is managed by the SDK).
func (s *Store) Close() error { return nil }

// isNotFound reports whether err represents a 404 / NoSuchKey response.
func isNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	// HeadObject returns a generic 404 rather than NoSuchKey.
	var notFound *types.NotFound
	return errors.As(err, &notFound)
}
