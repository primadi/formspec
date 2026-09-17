package garage

import (
	"context"
	"os"
	"testing"
)

// TestStorageUploadDownload verifies the Garage-backed Storage against a live
// Garage (devContainer: garage:3900). Skips when Garage is not reachable so
// CI without Garage stays green.
func TestStorageUploadDownload(t *testing.T) {
	endpoint := os.Getenv("FORMSPEC_GARAGE_ENDPOINT")
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	accessKey := os.Getenv("FORMSPEC_GARAGE_ACCESS_KEY")
	secretKey := os.Getenv("FORMSPEC_GARAGE_SECRET_KEY")
	bucket := os.Getenv("FORMSPEC_GARAGE_BUCKET")
	if bucket == "" {
		bucket = "formspec-test"
	}

	s, err := NewStorage(Config{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
	})
	if err != nil {
		t.Skipf("garage not reachable, skipping: %v", err)
	}

	ctx := context.Background()
	path := "test/hello.txt"
	if err := s.Upload(ctx, path, []byte("hello garage")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	data, err := s.Download(ctx, path)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if string(data) != "hello garage" {
		t.Fatalf("unexpected data %q", data)
	}
	if err := s.Delete(ctx, path); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
