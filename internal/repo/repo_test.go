package repo

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/index"
	"twig/internal/objects"
)

func TestRepoOpenAndAddFile(t *testing.T) {
	// Create temp directory for testing
	tmpDir, err := os.MkdirTemp("", "twig-repo-add-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Test Open on non-existent repo
	_, err = Open(tmpDir)
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("expected ErrNotARepo, got: %v", err)
	}

	// Initialize repo
	if err := Init(tmpDir); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	// Open initialized repo
	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	// 2. Test AddFile for a small file (Blob path)
	smallFileName := "small.txt"
	smallPath := filepath.Join(tmpDir, smallFileName)
	smallContent := []byte("hello blob world!")
	if err := os.WriteFile(smallPath, smallContent, 0644); err != nil {
		t.Fatalf("failed to write small file: %v", err)
	}

	if err := r.AddFile(smallPath); err != nil {
		t.Fatalf("AddFile failed on small file: %v", err)
	}

	// Verify index entry
	idx, err := index.Load(filepath.Join(r.TwigDir, "index"))
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	entry, ok := idx.Get(smallFileName)
	if !ok {
		t.Fatalf("expected entry %s in index", smallFileName)
	}

	if entry.Type != objects.TypeBlob {
		t.Errorf("expected type %s, got %s", objects.TypeBlob, entry.Type)
	}
	if entry.Size != int64(len(smallContent)) {
		t.Errorf("expected size %d, got %d", len(smallContent), entry.Size)
	}

	// Verify object exists in store
	exists, err := r.Store.Has(entry.Hash)
	if err != nil || !exists {
		t.Errorf("expected object %s to exist in store", entry.Hash)
	}

	// 3. Test AddFile for a large file (Asset path)
	// Needs to be >= 16KB
	largeSize := 20 * 1024
	largeContent := make([]byte, largeSize)
	rnd := rand.New(rand.NewSource(99))
	rnd.Read(largeContent)

	largeFileName := filepath.Join("sub", "dir", "large.bin")
	largePath := filepath.Join(tmpDir, largeFileName)
	if err := os.MkdirAll(filepath.Dir(largePath), 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	if err := os.WriteFile(largePath, largeContent, 0644); err != nil {
		t.Fatalf("failed to write large file: %v", err)
	}

	// Add file (test relative path resolution and OS slash normalization)
	if err := r.AddFile(largePath); err != nil {
		t.Fatalf("AddFile failed on large file: %v", err)
	}

	// Reload index
	idx, err = index.Load(filepath.Join(r.TwigDir, "index"))
	if err != nil {
		t.Fatalf("failed to reload index: %v", err)
	}

	// The index key should use forward slashes even on Windows
	normalizedName := "sub/dir/large.bin"
	entryLarge, ok := idx.Get(normalizedName)
	if !ok {
		t.Fatalf("expected entry %s in index", normalizedName)
	}

	if entryLarge.Type != objects.TypeAsset {
		t.Errorf("expected type %s, got %s", objects.TypeAsset, entryLarge.Type)
	}
	if entryLarge.Size != int64(largeSize) {
		t.Errorf("expected size %d, got %d", largeSize, entryLarge.Size)
	}

	// Verify asset manifest exists in store
	exists, err = r.Store.Has(entryLarge.Hash)
	if err != nil || !exists {
		t.Errorf("expected asset manifest %s to exist in store", entryLarge.Hash)
	}
}

func TestRepoAddDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-repo-dir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	// Create subdirectories and files
	aPath := filepath.Join(tmpDir, "a.txt")
	bPath := filepath.Join(tmpDir, "sub", "b.txt")
	emptyDirPath := filepath.Join(tmpDir, "sub", "empty_dir")
	cPath := filepath.Join(tmpDir, "sub", "deep", "c.bin")

	if err := os.MkdirAll(emptyDirPath, 0755); err != nil {
		t.Fatalf("failed to create subdir structure: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cPath), 0755); err != nil {
		t.Fatalf("failed to create deep subdir: %v", err)
	}

	if err := os.WriteFile(aPath, []byte("file A"), 0644); err != nil {
		t.Fatalf("failed to write A: %v", err)
	}
	if err := os.WriteFile(bPath, []byte("file B content"), 0644); err != nil {
		t.Fatalf("failed to write B: %v", err)
	}

	largeSize := 25 * 1024
	largeContent := make([]byte, largeSize)
	rnd := rand.New(rand.NewSource(123))
	rnd.Read(largeContent)
	if err := os.WriteFile(cPath, largeContent, 0644); err != nil {
		t.Fatalf("failed to write C: %v", err)
	}

	// Add the repository root directory
	if err := r.AddFile(tmpDir); err != nil {
		t.Fatalf("AddFile failed on directory: %v", err)
	}

	// Load index
	idx, err := index.Load(filepath.Join(r.TwigDir, "index"))
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	// Check files are correctly staged
	expectedFiles := []struct {
		name string
		typ  objects.ObjectType
		size int64
	}{
		{"a.txt", objects.TypeBlob, 6},
		{"sub/b.txt", objects.TypeBlob, 14},
		{"sub/deep/c.bin", objects.TypeAsset, int64(largeSize)},
	}

	for _, ef := range expectedFiles {
		entry, ok := idx.Get(ef.name)
		if !ok {
			t.Errorf("expected entry %s to be staged in index", ef.name)
			continue
		}
		if entry.Type != ef.typ {
			t.Errorf("expected %s to be %s, got %s", ef.name, ef.typ, entry.Type)
		}
		if entry.Size != ef.size {
			t.Errorf("expected %s size %d, got %d", ef.name, ef.size, entry.Size)
		}
	}

	// Ensure there are exactly 3 entries (meaning no empty directory, and no .twig entries)
	if len(idx.Entries) != 3 {
		t.Errorf("expected exactly 3 staged entries, got %d: %+v", len(idx.Entries), idx.Entries)
	}
}
