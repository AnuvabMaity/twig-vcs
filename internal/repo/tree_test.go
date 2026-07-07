package repo

import (
	"os"
	"testing"

	"twig/internal/index"
	"twig/internal/objects"
	"twig/internal/store"
)

func TestBuildTreeEmpty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-tree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure store layout: %v", err)
	}

	hash, err := BuildTree(s, nil)
	if err != nil {
		t.Fatalf("failed to build tree: %v", err)
	}

	data, err := s.Get(hash)
	if err != nil {
		t.Fatalf("failed to get tree object from store: %v", err)
	}

	var tree objects.Tree
	if err := objects.Decode(data, &tree); err != nil {
		t.Fatalf("failed to decode tree object: %v", err)
	}

	if len(tree.Entries) != 0 {
		t.Errorf("expected empty tree entries, got %d", len(tree.Entries))
	}
}

func TestBuildTreeDeterminism(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-tree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure store layout: %v", err)
	}

	entries1 := map[string]index.Entry{
		"src/main.go":  {Hash: "hash-main", Type: objects.TypeBlob, Size: 100, ModTime: 1},
		"src/pkg/a.go": {Hash: "hash-a", Type: objects.TypeBlob, Size: 200, ModTime: 2},
		"README.md":    {Hash: "hash-readme", Type: objects.TypeBlob, Size: 50, ModTime: 3},
	}

	entries2 := map[string]index.Entry{
		"README.md":    {Hash: "hash-readme", Type: objects.TypeBlob, Size: 50, ModTime: 3},
		"src/pkg/a.go": {Hash: "hash-a", Type: objects.TypeBlob, Size: 200, ModTime: 2},
		"src/main.go":  {Hash: "hash-main", Type: objects.TypeBlob, Size: 100, ModTime: 1},
	}

	hash1, err := BuildTree(s, entries1)
	if err != nil {
		t.Fatalf("failed to build tree 1: %v", err)
	}

	hash2, err := BuildTree(s, entries2)
	if err != nil {
		t.Fatalf("failed to build tree 2: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("BuildTree was not deterministic: hash1=%s, hash2=%s", hash1, hash2)
	}
}

func TestBuildTreeNestedStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-tree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure store layout: %v", err)
	}

	// Staging entries:
	// a.txt (Blob)
	// src/pkg/file.go (Blob)
	// src/README.md (Blob)
	entries := map[string]index.Entry{
		"a.txt":           {Hash: "hash-a", Type: objects.TypeBlob, Size: 10},
		"src/pkg/file.go": {Hash: "hash-file", Type: objects.TypeBlob, Size: 20},
		"src/README.md":   {Hash: "hash-readme", Type: objects.TypeBlob, Size: 30},
	}

	rootHash, err := BuildTree(s, entries)
	if err != nil {
		t.Fatalf("failed to build tree: %v", err)
	}

	// 1. Read and verify root Tree
	rootData, err := s.Get(rootHash)
	if err != nil {
		t.Fatalf("failed to get root tree: %v", err)
	}
	var rootTree objects.Tree
	if err := objects.Decode(rootData, &rootTree); err != nil {
		t.Fatalf("failed to decode root tree: %v", err)
	}

	if len(rootTree.Entries) != 2 {
		t.Fatalf("expected 2 entries in root tree, got %d", len(rootTree.Entries))
	}

	// Root entries should be sorted by Name: "a.txt", then "src"
	if rootTree.Entries[0].Name != "a.txt" || rootTree.Entries[0].Type != objects.TypeBlob {
		t.Errorf("invalid entry 0 in root: %+v", rootTree.Entries[0])
	}
	if rootTree.Entries[1].Name != "src" || rootTree.Entries[1].Type != objects.TypeTree {
		t.Errorf("invalid entry 1 in root: %+v", rootTree.Entries[1])
	}

	srcHash := rootTree.Entries[1].Hash

	// 2. Read and verify "src" Tree
	srcData, err := s.Get(srcHash)
	if err != nil {
		t.Fatalf("failed to get src tree: %v", err)
	}
	var srcTree objects.Tree
	if err := objects.Decode(srcData, &srcTree); err != nil {
		t.Fatalf("failed to decode src tree: %v", err)
	}

	if len(srcTree.Entries) != 2 {
		t.Fatalf("expected 2 entries in src tree, got %d", len(srcTree.Entries))
	}

	// Sort order: "README.md", then "pkg"
	if srcTree.Entries[0].Name != "README.md" || srcTree.Entries[0].Type != objects.TypeBlob {
		t.Errorf("invalid entry 0 in src: %+v", srcTree.Entries[0])
	}
	if srcTree.Entries[1].Name != "pkg" || srcTree.Entries[1].Type != objects.TypeTree {
		t.Errorf("invalid entry 1 in src: %+v", srcTree.Entries[1])
	}

	pkgHash := srcTree.Entries[1].Hash

	// 3. Read and verify "pkg" Tree
	pkgData, err := s.Get(pkgHash)
	if err != nil {
		t.Fatalf("failed to get pkg tree: %v", err)
	}
	var pkgTree objects.Tree
	if err := objects.Decode(pkgData, &pkgTree); err != nil {
		t.Fatalf("failed to decode pkg tree: %v", err)
	}

	if len(pkgTree.Entries) != 1 {
		t.Fatalf("expected 1 entry in pkg tree, got %d", len(pkgTree.Entries))
	}

	if pkgTree.Entries[0].Name != "file.go" || pkgTree.Entries[0].Type != objects.TypeBlob || pkgTree.Entries[0].Hash != "hash-file" {
		t.Errorf("invalid entry in pkg: %+v", pkgTree.Entries[0])
	}
}
