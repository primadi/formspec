// Package minio provides a MinIO/S3-backed object store implementing the
// same Upload/Download contract as ctx.storage() (and api.Storage). It is
// used for file fields (todo 7.17.1) when FORMSPEC_STORAGE=minio.
package minio

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

// Storage is a MinIO/S3-backed object store.
type Storage struct {
	client *minio.Client
	bucket string
}

// NewStorage creates a MinIO client, ensures the bucket exists, and returns
// a Storage rooted at that bucket. useSSL selects https vs http.
func NewStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Storage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check %q: %w", bucket, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket %q: %w", bucket, err)
		}
	}

	return &Storage{client: client, bucket: bucket}, nil
}

// Upload writes data to path.
func (s *Storage) Upload(ctx context.Context, path string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, path, bytes.NewReader(data),
		int64(len(data)), minio.PutObjectOptions{ContentType: contentTypeFor(path)})
	if err != nil {
		return fmt.Errorf("minio upload %s: %w", path, err)
	}
	return nil
}

// Download reads data from path.
func (s *Storage) Download(ctx context.Context, path string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("minio download %s: %w", path, err)
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("minio read %s: %w", path, err)
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
