package store

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestStorePutGetHas(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-store-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := Open(tmpDir)
	err = s.EnsureLayout()
	if err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	content := []byte("hello store content")
	hash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // dummy hash value (actually this will be computed by blake3)

	// Verify Has is false before Put
	has, err := s.Has(hash)
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if has {
		t.Errorf("Expected Has to be false for non-existent object")
	}

	// Put content
	realHash, err := s.Put(content)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify Has is true after Put
	has, err = s.Has(realHash)
	if err != nil {
		t.Fatalf("Has failed: %v", err)
	}
	if !has {
		t.Errorf("Expected Has to be true after Put")
	}

	// Get content back
	retrieved, err := s.Get(realHash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !bytes.Equal(content, retrieved) {
		t.Errorf("Expected retrieved content to match original. Expected %q, got %q", content, retrieved)
	}

	// Deduplication test: Put same content again
	realHash2, err := s.Put(content)
	if err != nil {
		t.Fatalf("Second Put failed: %v", err)
	}

	if realHash != realHash2 {
		t.Errorf("Expected identical hashes for identical content, got %s and %s", realHash, realHash2)
	}

	// Count files under objects directory to ensure only one exists
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

func TestStoreGetNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-store-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := Open(tmpDir)
	_, err = s.Get("notahash12345")
	if err == nil {
		t.Errorf("Expected error when getting non-existent hash, got nil")
	}
}
