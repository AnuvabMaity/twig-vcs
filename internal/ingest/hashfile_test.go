package ingest

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/store"
)

func countObjects(t *testing.T, objectsDir string) int {
	count := 0
	if _, err := os.Stat(objectsDir); os.IsNotExist(err) {
		return 0
	}
	err := filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk objects dir: %v", err)
	}
	return count
}

func TestHashFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-hashfile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	objectsDir := filepath.Join(tmpDir, "objects")

	// 1. Create a small file (< 16KB)
	smallPath := filepath.Join(tmpDir, "small.txt")
	smallContent := []byte("small file content hello world")
	if err := os.WriteFile(smallPath, smallContent, 0644); err != nil {
		t.Fatalf("failed to write small file: %v", err)
	}

	// Count files in store before HashFile
	countBeforeSmall := countObjects(t, objectsDir)

	// Call HashFile
	smallHashComputed, smallTypeComputed, err := HashFile(smallPath)
	if err != nil {
		t.Fatalf("HashFile failed for small file: %v", err)
	}

	// Verify no objects were written
	countAfterSmall := countObjects(t, objectsDir)
	if countAfterSmall != countBeforeSmall {
		t.Errorf("expected object count to remain %d, got %d", countBeforeSmall, countAfterSmall)
	}

	// Now Ingest the file and compare hashes
	smallHashIngested, smallTypeIngested, err := IngestFile(s, smallPath)
	if err != nil {
		t.Fatalf("IngestFile failed for small file: %v", err)
	}

	if smallHashComputed != smallHashIngested {
		t.Errorf("hash mismatch for small file: computed %s, ingested %s", smallHashComputed, smallHashIngested)
	}
	if smallTypeComputed != smallTypeIngested {
		t.Errorf("type mismatch for small file: computed %s, ingested %s", smallTypeComputed, smallTypeIngested)
	}

	// 2. Create a large file (>= 16KB, e.g. 500KB to make sure chunking is triggered)
	largePath := filepath.Join(tmpDir, "large.bin")
	largeContent := make([]byte, 500*1024)
	r := rand.New(rand.NewSource(99))
	if _, err := r.Read(largeContent); err != nil {
		t.Fatalf("failed to read random bytes: %v", err)
	}
	if err := os.WriteFile(largePath, largeContent, 0644); err != nil {
		t.Fatalf("failed to write large file: %v", err)
	}

	// Count files in store before HashFile
	countBeforeLarge := countObjects(t, objectsDir)

	// Call HashFile
	largeHashComputed, largeTypeComputed, err := HashFile(largePath)
	if err != nil {
		t.Fatalf("HashFile failed for large file: %v", err)
	}

	// Verify no objects were written
	countAfterLarge := countObjects(t, objectsDir)
	if countAfterLarge != countBeforeLarge {
		t.Errorf("expected object count to remain %d, got %d", countBeforeLarge, countAfterLarge)
	}

	// Now Ingest the file and compare hashes
	largeHashIngested, largeTypeIngested, err := IngestFile(s, largePath)
	if err != nil {
		t.Fatalf("IngestFile failed for large file: %v", err)
	}

	if largeHashComputed != largeHashIngested {
		t.Errorf("hash mismatch for large file: computed %s, ingested %s", largeHashComputed, largeHashIngested)
	}
	if largeTypeComputed != largeTypeIngested {
		t.Errorf("type mismatch for large file: computed %s, ingested %s", largeTypeComputed, largeTypeIngested)
	}
}
