package minio

import (
	"context"
	"os"
	"testing"
)

// TestStorageUploadDownload verifies the MinIO-backed Storage against a live
// MinIO (devContainer: minio:9000, minioadmin/minioadmin). Skips when MinIO
// is not reachable so CI without MinIO stays green.
func TestStorageUploadDownload(t *testing.T) {
	endpoint := os.Getenv("FORMSPEC_MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "minio:9000"
	}
	accessKey := os.Getenv("FORMSPEC_MINIO_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "minioadmin"
	}
	secretKey := os.Getenv("FORMSPEC_MINIO_SECRET_KEY")
	if secretKey == "" {
		secretKey = "minioadmin"
	}
	bucket := os.Getenv("FORMSPEC_MINIO_BUCKET")
	if bucket == "" {
		bucket = "formspec-test"
	}

	s, err := NewStorage(endpoint, accessKey, secretKey, bucket, false)
	if err != nil {
		t.Skipf("minio not reachable, skipping: %v", err)
	}

	ctx := context.Background()
	path := "test/hello.txt"
	if err := s.Upload(ctx, path, []byte("hello minio")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	data, err := s.Download(ctx, path)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(data) != "hello minio" {
		t.Fatalf("unexpected data %q", data)
	}
}
