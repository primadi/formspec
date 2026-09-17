// Package s3store is an S3-compatible object store (AWS Signature v4) that
// implements the same Upload/Download contract as ctx.storage() (and
// api.Storage), plus the extended capabilities Stat/Delete/Link/ChunkUpload.
//
// It is the shared implementation behind the `garage` (default) and `minio`
// datastore drivers: both speak the S3 API, so the client is identical and
// only the defaults (endpoint port, region) differ. The driver packages
// (datastore/garage, datastore/minio) are thin wrappers that pin those
// defaults.
package s3store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// DefaultRegion is used when Config.Region is empty. It is minio-go's own
// default, so an S3-compatible server configured with `s3_region =
// "us-east-1"` works out of the box.
const DefaultRegion = "us-east-1"

// Config holds the connection parameters of an S3-compatible object store.
type Config struct {
	// Endpoint is host:port (no scheme), e.g. "garage:3900".
	Endpoint string

	// AccessKey / SecretKey are the S3 credentials.
	AccessKey string
	SecretKey string

	// Bucket is the target bucket; it is created when missing.
	Bucket string

	// Region must match the server's configured S3 region. Empty means
	// DefaultRegion.
	Region string

	// UseSSL selects https vs http.
	UseSSL bool
}

// Storage is an S3-compatible object store.
type Storage struct {
	client *minio.Client
	core   minio.Core // multipart upload API (PutObjectPart lives on Core)
	bucket string
}

// New creates an S3 client, ensures the bucket exists, and returns a Storage
// rooted at that bucket. Path-style addressing is forced because it is
// supported by every S3-compatible server (Garage and MinIO included) while
// vhost-style needs wildcard DNS.
func New(cfg Config) (*Storage, error) {
	region := cfg.Region
	if region == "" {
		region = DefaultRegion
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       cfg.UseSSL,
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("object store client %s: %w", cfg.Endpoint, err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("object store bucket check %q: %w", cfg.Bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			return nil, fmt.Errorf("object store make bucket %q: %w", cfg.Bucket, err)
		}
	}

	return &Storage{client: client, core: minio.Core{Client: client}, bucket: cfg.Bucket}, nil
}

// Bucket returns the bucket this storage is rooted at.
func (s *Storage) Bucket() string { return s.bucket }

// Upload writes data to path.
func (s *Storage) Upload(ctx context.Context, path string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, path, bytes.NewReader(data),
		int64(len(data)), minio.PutObjectOptions{ContentType: contentTypeFor(path)})
	if err != nil {
		return fmt.Errorf("object store upload %s: %w", path, err)
	}
	return nil
}

// Download reads data from path.
func (s *Storage) Download(ctx context.Context, path string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("object store download %s: %w", path, err)
	}
	defer func() { _ = obj.Close() }()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("object store read %s: %w", path, err)
	}
	return data, nil
}

// contentTypeFor maps a path's extension to a MIME type for object metadata.
func contentTypeFor(path string) string {
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
