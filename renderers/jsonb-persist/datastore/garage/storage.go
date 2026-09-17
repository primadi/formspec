// Package garage provides a Garage-backed object store implementing the same
// Upload/Download contract as ctx.storage() (and api.Storage), used for file
// fields when a `kind: Datastore` declares `driver: garage`.
//
// Garage (https://garagehq.deuxfleurs.fr) is the default S3-compatible object
// storage driver: a self-hosted, geo-distributed object store. The dev
// container runs a single-node Garage whose S3 API listens on :3900.
//
// Garage speaks the S3 API, so the client is the shared datastore/s3store
// implementation — this package pins the Garage defaults (S3 port 3900,
// default bucket, region) on top of it.
package garage

import (
	"fmt"

	"github.com/primadi/formspec/renderers/jsonb-persist/datastore/s3store"
)

const (
	// DefaultHost is the dev container service name.
	DefaultHost = "garage"

	// DefaultS3Port is Garage's S3 API port (`[s3_api] api_bind_addr`).
	DefaultS3Port = 3900

	// DefaultEndpoint is the dev container S3 endpoint.
	DefaultEndpoint = "garage:3900"

	// DefaultBucket is used when a Datastore declares no bucket.
	DefaultBucket = "formspec"

	// DefaultRegion must match Garage's `s3_region` (see garage.toml).
	DefaultRegion = "us-east-1"
)

// Config / Storage mirror the shared S3 client.
type (
	// Config holds the connection parameters.
	Config = s3store.Config
	// Storage is a Garage-backed object store; it carries the full S3
	// capability set (Stat/Delete/Link/ChunkUpload).
	Storage = s3store.Storage
)

// DefaultLinkTTL is used when a link request carries no explicit TTL.
const DefaultLinkTTL = s3store.DefaultLinkTTL

// Endpoint builds an S3 endpoint from a host and port, falling back to the
// Garage dev defaults.
func Endpoint(host string, port int) string {
	if host == "" {
		host = DefaultHost
	}
	if port <= 0 {
		port = DefaultS3Port
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// NewStorage opens a Garage-backed object store, applying the Garage defaults
// for Region and Bucket when the caller leaves them empty. Path-style
// addressing is used (Garage always accepts it; vhost-style needs wildcard
// DNS).
func NewStorage(cfg Config) (*Storage, error) {
	if cfg.Region == "" {
		cfg.Region = DefaultRegion
	}
	if cfg.Bucket == "" {
		cfg.Bucket = DefaultBucket
	}
	return s3store.New(cfg)
}
