package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadMediaSkipsExistingFile(t *testing.T) {
	oldFetchMedia := fetchMedia
	t.Cleanup(func() {
		fetchMedia = oldFetchMedia
	})

	dir := t.TempDir()
	filePath := filepath.Join(dir, "exists.jpeg")
	if err := os.WriteFile(filePath, []byte("cached"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	fetchMedia = func(string) ([]byte, error) {
		t.Fatal("fetchMedia should not be called for existing file")
		return nil, nil
	}

	got, err := downloadMedia("https://example.com/exists.jpeg", dir, "exists.jpeg")
	if err != nil {
		t.Fatalf("downloadMedia() error = %v", err)
	}
	if got != filePath {
		t.Fatalf("downloadMedia() path = %s, want %s", got, filePath)
	}
}

func TestDownloadMediaFetchesMissingFile(t *testing.T) {
	oldFetchMedia := fetchMedia
	t.Cleanup(func() {
		fetchMedia = oldFetchMedia
	})

	fetchMedia = func(string) ([]byte, error) {
		return []byte("downloaded"), nil
	}

	dir := t.TempDir()
	got, err := downloadMedia("https://example.com/new.jpeg", dir, "new.jpeg")
	if err != nil {
		t.Fatalf("downloadMedia() error = %v", err)
	}
	if got != filepath.Join(dir, "new.jpeg") {
		t.Fatalf("downloadMedia() path = %s", got)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "downloaded" {
		t.Fatalf("downloaded content = %q", string(data))
	}
}

func TestDownloadMediaReturnsFetchError(t *testing.T) {
	oldFetchMedia := fetchMedia
	t.Cleanup(func() {
		fetchMedia = oldFetchMedia
	})

	fetchMedia = func(string) ([]byte, error) {
		return nil, errors.New("timeout")
	}

	if _, err := downloadMedia("https://example.com/fail.jpeg", t.TempDir(), "fail.jpeg"); err == nil {
		t.Fatal("downloadMedia() error = nil, want timeout")
	}
}
