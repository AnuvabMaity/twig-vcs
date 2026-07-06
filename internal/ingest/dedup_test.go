package ingest

import (
	"bytes"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/store"
)

func TestCDCAppendOnlyDeduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-dedup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 1. Generate large random buffer (1.5MB)
	size1 := 1536 * 1024
	buf1 := make([]byte, size1)
	r := rand.New(rand.NewSource(42))
	if _, err := r.Read(buf1); err != nil {
		t.Fatalf("Failed to generate random buffer: %v", err)
	}

	// Ingest buf1
	_, err = BuildAsset(s, bytes.NewReader(buf1))
	if err != nil {
		t.Fatalf("BuildAsset failed: %v", err)
	}

	// Count number of files currently in the store
	objectsDir := filepath.Join(tmpDir, "objects")
	filesCount1 := 0
	err = filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			filesCount1++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	// 2. Append small number of bytes (100 bytes) to the end
	appendSize := 100
	appendBuf := make([]byte, appendSize)
	if _, err := r.Read(appendBuf); err != nil {
		t.Fatalf("Failed to generate append buffer: %v", err)
	}
	buf2 := append(buf1, appendBuf...)

	// Ingest buf2
	hash2, err := BuildAsset(s, bytes.NewReader(buf2))
	if err != nil {
		t.Fatalf("BuildAsset failed: %v", err)
	}

	// Count number of files in the store after second ingest
	filesCount2 := 0
	err = filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			filesCount2++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	newFilesWritten := filesCount2 - filesCount1

	// Log before/after details for visibility
	t.Logf("Initial files in store: %d (chunks + manifest)", filesCount1)
	t.Logf("Files in store after append: %d (chunks + manifests)", filesCount2)
	t.Logf("Newly written files: %d (expected 1 new chunk + 1 new manifest = 2, or at most 3)", newFilesWritten)

	// Assert that newly written files is at most 3 (at most 2 chunks + 1 manifest)
	if newFilesWritten > 3 {
		t.Errorf("Deduplication inefficient: expected at most 3 newly written files, got %d", newFilesWritten)
	}

	// Assert that reconstructed version of buf2 matches buf2 byte-for-byte
	var reconstructBuf bytes.Buffer
	if err := Reconstruct(s, hash2, "asset", &reconstructBuf); err != nil {
		t.Fatalf("Reconstruct failed: %v", err)
	}

	if !bytes.Equal(buf2, reconstructBuf.Bytes()) {
		t.Errorf("Reconstructed content mismatch for appended version")
	}
}
