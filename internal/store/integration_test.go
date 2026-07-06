package store

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/hashing"
	"twig/internal/objects"
)

func TestDeduplicationEndToEnd(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	blob := objects.Blob{
		Type: objects.TypeBlob,
		Data: []byte("identical test data content"),
	}

	encoded, err := objects.Encode(blob)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	hash1, err := s.Put(encoded)
	if err != nil {
		t.Fatalf("First Put failed: %v", err)
	}

	hash2, err := s.Put(encoded)
	if err != nil {
		t.Fatalf("Second Put failed: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Expected identical hashes, got %s and %s", hash1, hash2)
	}

	// Verify exactly one file exists on disk
	objectsDir := filepath.Join(tmpDir, "objects")
	fileCount := 0
	err = filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	if fileCount != 1 {
		t.Errorf("Expected exactly 1 object file under objects directory, found %d", fileCount)
	}
}

func TestCorruptedObjectDetection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	blob := objects.Blob{
		Type: objects.TypeBlob,
		Data: []byte("corruption test content"),
	}

	encoded, err := objects.Encode(blob)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	hash, err := s.Put(encoded)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Find the on-disk file path
	objectFile := hashing.ObjectPath(tmpDir, hash)

	// Corrupt the file by truncating it to 5 bytes
	if err := os.WriteFile(objectFile, []byte("bad!!"), 0644); err != nil {
		t.Fatalf("Failed to write corrupted data to object file: %v", err)
	}

	// Read corrupted object
	_, err = s.Get(hash)
	if err == nil {
		t.Errorf("Expected Get to fail on corrupted/truncated file, but returned nil error")
	}
}

func TestObjectNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-integration-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	randomHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	_, err = s.Get(randomHash)
	if err == nil {
		t.Errorf("Expected error for non-existent hash, got nil")
	}
}
