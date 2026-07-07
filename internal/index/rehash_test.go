package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"twig/internal/objects"
)

func TestNeedsRehash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-rehash-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	fi, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	entry := Entry{
		Hash:    "dummyhash",
		Type:    objects.TypeBlob,
		Size:    fi.Size(),
		ModTime: fi.ModTime().UnixNano(),
	}

	// 1. Unmodified file -> NeedsRehash should return false
	needsRehash, err := NeedsRehash(filePath, entry)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if needsRehash {
		t.Error("expected needsRehash to be false for unmodified file")
	}

	// 2. Changed size -> NeedsRehash should return true
	entrySizeDiff := entry
	entrySizeDiff.Size = 9999
	needsRehash, err = NeedsRehash(filePath, entrySizeDiff)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !needsRehash {
		t.Error("expected needsRehash to be true when size differs")
	}

	// 3. Changed mtime -> NeedsRehash should return true
	entryMtimeDiff := entry
	entryMtimeDiff.ModTime = fi.ModTime().Add(-1 * time.Hour).UnixNano()
	needsRehash, err = NeedsRehash(filePath, entryMtimeDiff)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !needsRehash {
		t.Error("expected needsRehash to be true when mtime differs")
	}

	// 4. Nonexistent file -> NeedsRehash should return an error
	_, err = NeedsRehash(filepath.Join(tmpDir, "nonexistent.txt"), entry)
	if err == nil {
		t.Error("expected NeedsRehash to fail for nonexistent file")
	}
}
