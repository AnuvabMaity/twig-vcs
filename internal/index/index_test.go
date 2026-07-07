package index

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"twig/internal/objects"
)

func TestIndexLoadNonexistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-index-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexPath := filepath.Join(tmpDir, "nonexistent-index")
	idx, err := Load(indexPath)
	if err != nil {
		t.Fatalf("Load failed for nonexistent path: %v", err)
	}
	if idx == nil {
		t.Fatal("Load returned nil index")
	}
	if idx.Entries == nil {
		t.Fatal("Load returned index with nil Entries map")
	}
	if len(idx.Entries) != 0 {
		t.Errorf("expected empty index, got %d entries", len(idx.Entries))
	}
}

func TestIndexPutGetRemove(t *testing.T) {
	idx := &Index{
		Entries: make(map[string]Entry),
	}

	path := "src/main.go"
	entry := Entry{
		Hash:    "dummyhash123",
		Type:    objects.TypeBlob,
		Size:    1234,
		ModTime: 1672531199000000000,
	}

	// Get absent
	_, ok := idx.Get(path)
	if ok {
		t.Fatal("expected Get to fail for absent path")
	}

	// Remove absent
	idx.Remove(path) // should not panic/error

	// Put
	idx.Put(path, entry)

	// Get present
	retrieved, ok := idx.Get(path)
	if !ok {
		t.Fatal("expected Get to find path")
	}
	if !reflect.DeepEqual(retrieved, entry) {
		t.Errorf("expected entry %+v, got %+v", entry, retrieved)
	}

	// Remove present
	idx.Remove(path)
	_, ok = idx.Get(path)
	if ok {
		t.Fatal("expected path to be removed")
	}
}

func TestIndexSaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-index-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	idx := &Index{
		Entries: make(map[string]Entry),
	}

	entry1 := Entry{
		Hash:    "hash1",
		Type:    objects.TypeBlob,
		Size:    100,
		ModTime: 123456789,
	}
	entry2 := Entry{
		Hash:    "hash2",
		Type:    objects.TypeAsset,
		Size:    50000,
		ModTime: 987654321,
	}

	idx.Put("file1.txt", entry1)
	idx.Put("sub/file2.bin", entry2)

	indexPath := filepath.Join(tmpDir, "test-index")
	if err := idx.Save(indexPath); err != nil {
		t.Fatalf("failed to save index: %v", err)
	}

	loaded, err := Load(indexPath)
	if err != nil {
		t.Fatalf("failed to load saved index: %v", err)
	}

	if !reflect.DeepEqual(idx, loaded) {
		t.Errorf("round-trip mismatch: expected %+v, got %+v", idx, loaded)
	}
}
