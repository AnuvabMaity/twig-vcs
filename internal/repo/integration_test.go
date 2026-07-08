package repo

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/index"
	"twig/internal/ingest"
	"twig/internal/objects"
	"twig/internal/refs"
	"twig/internal/store"
)

func TestPhase3Integration(t *testing.T) {
	// 1. Create a temporary directory and call repo.Init
	tmpDir, err := os.MkdirTemp("", "twig-ph3-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 2. Open the repo
	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// 3. Create fixture tree
	// - small1.txt: 100 bytes (Blob)
	// - small2.txt: 5KB (Blob)
	// - sub/large.bin: 1.2MB (Asset, over 1MB size)
	small1Path := filepath.Join(tmpDir, "small1.txt")
	small2Path := filepath.Join(tmpDir, "small2.txt")
	subDir := filepath.Join(tmpDir, "sub")
	largePath := filepath.Join(subDir, "large.bin")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	small1Content := []byte("This is a small file content to test blob path of Phase 3 integration testing.")
	if err := os.WriteFile(small1Path, small1Content, 0644); err != nil {
		t.Fatalf("failed to write small1: %v", err)
	}

	small2Content := make([]byte, 5*1024)
	rnd := rand.New(rand.NewSource(12345))
	rnd.Read(small2Content)
	if err := os.WriteFile(small2Path, small2Content, 0644); err != nil {
		t.Fatalf("failed to write small2: %v", err)
	}

	largeSize := 1200 * 1024 // 1.2 MB
	largeContent := make([]byte, largeSize)
	rnd.Read(largeContent)
	if err := os.WriteFile(largePath, largeContent, 0644); err != nil {
		t.Fatalf("failed to write large file: %v", err)
	}

	// 4. Call AddFile on the whole tree (the root tmpDir)
	if err := r.AddFile(tmpDir); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	// 5. Reload index from disk
	indexPath := filepath.Join(r.TwigDir, "index")
	idx, err := index.Load(indexPath)
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}

	// 6. Set up a throwaway store to independently calculate hashes via IngestFile
	throwawayDir, err := os.MkdirTemp("", "twig-throwaway-*")
	if err != nil {
		t.Fatalf("failed to create throwaway temp dir: %v", err)
	}
	defer os.RemoveAll(throwawayDir)

	throwawayStore := store.Open(throwawayDir)
	if err := throwawayStore.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure throwaway layout: %v", err)
	}

	expectedFiles := []struct {
		relPath string
		absPath string
		typ     objects.ObjectType
	}{
		{"small1.txt", small1Path, objects.TypeBlob},
		{"small2.txt", small2Path, objects.TypeBlob},
		{"sub/large.bin", largePath, objects.TypeAsset},
	}

	// Assert every file is present with the correct type and independently verified hash
	for _, ef := range expectedFiles {
		entry, ok := idx.Get(ef.relPath)
		if !ok {
			t.Errorf("expected file %s to be present in the loaded index", ef.relPath)
			continue
		}

		if entry.Type != ef.typ {
			t.Errorf("file %s: expected type %s, got %s", ef.relPath, ef.typ, entry.Type)
		}

		// Independently compute hash using IngestFile in throwaway store
		indepHash, indepType, err := ingest.IngestFile(throwawayStore, ef.absPath)
		if err != nil {
			t.Errorf("failed to independently ingest file %s: %v", ef.relPath, err)
			continue
		}

		if indepType != ef.typ {
			t.Errorf("file %s: independent ingest type %s did not match expected %s", ef.relPath, indepType, ef.typ)
		}

		if entry.Hash != indepHash {
			t.Errorf("file %s: hash in index %s did not match independently calculated hash %s", ef.relPath, entry.Hash, indepHash)
		}
	}

	// Ensure no extra files (e.g. .twig files) got added
	if len(idx.Entries) != len(expectedFiles) {
		t.Errorf("expected exactly %d staged files, got %d", len(expectedFiles), len(idx.Entries))
	}
}

func TestPhase4Integration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-ph4-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Write mock config so Commit() does not fail
	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	cfg := map[string]string{"user.id": "integration-tester"}
	if err := objects.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// 1. Add first file
	f1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(f1, []byte("content 1"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := r.AddFile(f1); err != nil {
		t.Fatalf("AddFile file1 failed: %v", err)
	}

	// Commit 1
	c1Hash, err := r.Commit("first commit")
	if err != nil {
		t.Fatalf("commit 1 failed: %v", err)
	}

	// 2. Add second file
	f2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(f2, []byte("content 2"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}
	if err := r.AddFile(f2); err != nil {
		t.Fatalf("AddFile file2 failed: %v", err)
	}

	// Commit 2
	c2Hash, err := r.Commit("second commit")
	if err != nil {
		t.Fatalf("commit 2 failed: %v", err)
	}

	// 3. Assert ResolveHEAD matches second commit's hash
	headHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		t.Fatalf("ResolveHEAD failed: %v", err)
	}
	if headHash != c2Hash {
		t.Errorf("expected ResolveHEAD to return second commit hash %s, got %s", c2Hash, headHash)
	}

	// 4. Assert decoding second commit shows first commit's hash as parent
	c2Bytes, err := r.Store.Get(c2Hash)
	if err != nil {
		t.Fatalf("failed to get commit 2 from store: %v", err)
	}
	var commit2 objects.Commit
	if err := objects.Decode(c2Bytes, &commit2); err != nil {
		t.Fatalf("failed to decode commit 2: %v", err)
	}
	if len(commit2.Parents) != 1 || commit2.Parents[0] != c1Hash {
		t.Errorf("expected parent to be first commit %s, got parent list %+v", c1Hash, commit2.Parents)
	}

	// 5. Assert Repo.Log returns both commits in the correct order with correct messages
	entries, err := r.Log(c2Hash)
	if err != nil {
		t.Fatalf("r.Log failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}

	// Newest first order: commit 2, then commit 1
	if entries[0].Hash != c2Hash || entries[0].Commit.Message != "second commit" {
		t.Errorf("entry 0 invalid: %+v", entries[0])
	}
	if entries[1].Hash != c1Hash || entries[1].Commit.Message != "first commit" {
		t.Errorf("entry 1 invalid: %+v", entries[1])
	}
}
