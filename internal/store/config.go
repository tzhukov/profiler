package store

// Config holds configuration for both blob and index backends.
// Populate only the fields relevant to the chosen backend.
type Config struct {
	// BlobBackend selects the blob storage implementation.
	// Valid values: "badger", "s3"
	BlobBackend string

	// BadgerPath is the on-disk directory for the Badger key-value store.
	// Required when BlobBackend == "badger".
	BadgerPath string

	// S3Endpoint is the HTTP(S) URL of the S3 (or MinIO) endpoint.
	// Required when BlobBackend == "s3".
	// Example: "https://s3.us-east-1.amazonaws.com" or "http://minio:9000"
	S3Endpoint string

	// S3Bucket is the name of the bucket that will hold profile blobs.
	// Required when BlobBackend == "s3".
	S3Bucket string

	// S3Region is the AWS region.
	// Optional for MinIO; required for AWS S3.
	S3Region string

	// S3AccessKey / S3SecretKey are static credentials.
	// Leave blank to fall back to the default credential chain
	// (env vars, ~/.aws/credentials, instance metadata, etc.).
	S3AccessKey string
	S3SecretKey string

	// S3ForcePathStyle uses path-style addressing (required for MinIO).
	S3ForcePathStyle bool
}
