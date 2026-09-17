// Package minio provides a MinIO-backed object store implementing the same
// Upload/Download contract as ctx.storage() (and api.Storage), used for file
// fields when a `kind: Datastore` declares `driver: minio`.
//
// Garage (`driver: garage`) is the default object storage driver; this driver
// stays for deployments that already run MinIO. MinIO speaks the S3 API, so
// the client is the shared datastore/s3store implementation — this package
// pins the MinIO defaults on top of it.
package minio

import (
	"fmt"

	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/s3store"
)

const (
	// DefaultHost is the dev container service name.
	DefaultHost = "minio"

	// DefaultS3Port is MinIO's S3 API port.
	DefaultS3Port = 9000

	// DefaultEndpoint is MinIO's dev container S3 endpoint.
	DefaultEndpoint = "minio:9000"

	// DefaultBucket is used when a Datastore declares no bucket.
	DefaultBucket = "formspec"

	// DefaultRegion must match MinIO's configured region.
	DefaultRegion = "us-east-1"
)

// Config / Storage mirror the shared S3 client.
type (
	// Config holds the connection parameters.
	Config = s3store.Config
	// Storage is a MinIO-backed object store; it carries the full S3
	// capability set (Stat/Delete/Link/ChunkUpload).
	Storage = s3store.Storage
)

// DefaultLinkTTL is used when a link request carries no explicit TTL.
const DefaultLinkTTL = s3store.DefaultLinkTTL

// Endpoint builds an S3 endpoint from a host and port, falling back to the
// MinIO dev defaults.
func Endpoint(host string, port int) string {
	if host == "" {
		host = DefaultHost
	}
	if port <= 0 {
		port = DefaultS3Port
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// NewStorage opens a MinIO-backed object store, applying the MinIO defaults
// for Region and Bucket when the caller leaves them empty.
func NewStorage(cfg Config) (*Storage, error) {
	if cfg.Region == "" {
		cfg.Region = DefaultRegion
	}
	if cfg.Bucket == "" {
		cfg.Bucket = DefaultBucket
	}
	return s3store.New(cfg)
}
