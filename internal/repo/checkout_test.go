package repo

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/index"
	"twig/internal/ingest"
	"twig/internal/objects"
	"twig/internal/store"
)

func TestWalkTreeEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-checkout-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure layout: %v", err)
	}

	// 1. Build an empty tree
	emptyTreeHash, err := BuildTree(s, nil)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	// 2. Walk the empty tree
	files, err := WalkTree(s, emptyTreeHash)
	if err != nil {
		t.Fatalf("WalkTree failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files from empty tree walk, got %d", len(files))
	}
}

func TestWalkTreeNested(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-checkout-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure layout: %v", err)
	}

	// 1. Prepare simulated index entries
	entries := map[string]index.Entry{
		"a.txt":         {Hash: "hash-a", Type: objects.TypeBlob},
		"src/b.txt":     {Hash: "hash-b", Type: objects.TypeBlob},
		"src/pkg/c.bin": {Hash: "hash-c", Type: objects.TypeAsset},
	}

	// 2. Build the nested tree hierarchy
	rootTreeHash, err := BuildTree(s, entries)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	// 3. Walk the tree
	files, err := WalkTree(s, rootTreeHash)
	if err != nil {
		t.Fatalf("WalkTree failed: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	expected := []struct {
		path string
		hash string
		typ  objects.ObjectType
	}{
		{"a.txt", "hash-a", objects.TypeBlob},
		{"src/b.txt", "hash-b", objects.TypeBlob},
		{"src/pkg/c.bin", "hash-c", objects.TypeAsset},
	}

	for i, exp := range expected {
		if files[i].Path != exp.path {
			t.Errorf("file %d: expected path %q, got %q", i, exp.path, files[i].Path)
		}
		if files[i].Hash != exp.hash {
			t.Errorf("file %d: expected hash %q, got %q", i, exp.hash, files[i].Hash)
		}
		if files[i].Type != exp.typ {
			t.Errorf("file %d: expected type %s, got %s", i, exp.typ, files[i].Type)
		}
	}
}

func TestWriteWorkingDir(t *testing.T) {
	// Create temporary directory for the object store
	storeDir, err := os.MkdirTemp("", "twig-store-*")
	if err != nil {
		t.Fatalf("failed to create temp store dir: %v", err)
	}
	defer os.RemoveAll(storeDir)

	s := store.Open(storeDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure layout: %v", err)
	}

	// Create temporary directory for source files
	srcDir, err := os.MkdirTemp("", "twig-src-*")
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// 1. Create a small file (Blob)
	smallPath := filepath.Join(srcDir, "small.txt")
	smallContent := []byte("small file content")
	if err := os.WriteFile(smallPath, smallContent, 0644); err != nil {
		t.Fatalf("failed to write small file: %v", err)
	}

	// 2. Create a large file (Asset, multi-chunk, e.g. 1.2MB)
	largePath := filepath.Join(srcDir, "large.bin")
	largeSize := 1200 * 1024
	largeContent := make([]byte, largeSize)
	rnd := rand.New(rand.NewSource(99))
	rnd.Read(largeContent)
	if err := os.WriteFile(largePath, largeContent, 0644); err != nil {
		t.Fatalf("failed to write large file: %v", err)
	}

	// Ingest files into the store to generate hashes and objects
	smallHash, smallType, err := ingest.IngestFile(s, smallPath)
	if err != nil {
		t.Fatalf("failed to ingest small file: %v", err)
	}
	largeHash, largeType, err := ingest.IngestFile(s, largePath)
	if err != nil {
		t.Fatalf("failed to ingest large file: %v", err)
	}

	// 3. Prepare TreeFile list to write to a brand new root that doesn't exist yet
	destRoot := filepath.Join(srcDir, "dest-root-new")

	files := []TreeFile{
		{
			Path: "a.txt",
			Hash: smallHash,
			Type: smallType,
		},
		{
			Path: "src/pkg/c.bin",
			Hash: largeHash,
			Type: largeType,
		},
	}

	// 4. Write working directory (creates destRoot automatically)
	if err := WriteWorkingDir(s, destRoot, files); err != nil {
		t.Fatalf("WriteWorkingDir failed: %v", err)
	}

	// 5. Verify reconstructed files
	// Verify small.txt
	reconSmallPath := filepath.Join(destRoot, "a.txt")
	reconSmallContent, err := os.ReadFile(reconSmallPath)
	if err != nil {
		t.Fatalf("failed to read reconstructed small file: %v", err)
	}
	if !bytes.Equal(reconSmallContent, smallContent) {
		t.Errorf("reconstructed small content mismatch")
	}

	// Verify large.bin (nested subdirectory src/pkg/ was created automatically)
	reconLargePath := filepath.Join(destRoot, "src", "pkg", "c.bin")
	reconLargeContent, err := os.ReadFile(reconLargePath)
	if err != nil {
		t.Fatalf("failed to read reconstructed large file: %v", err)
	}
	if !bytes.Equal(reconLargeContent, largeContent) {
		t.Errorf("reconstructed large content mismatch")
	}
}
