package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/objects"
	"twig/internal/refs"
)

func TestRepoCommitWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-repo-commit-test-*")
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

	// Mock configuration with user identity
	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	cfg := map[string]string{"user.id": "tester"}
	if err := objects.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// 1. First commit with no changes staged -> should succeed because index is empty (meaning empty root tree)
	c1Hash, err := r.Commit("initial commit")
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}

	// Resolve HEAD
	headHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		t.Fatalf("ResolveHEAD failed: %v", err)
	}
	if headHash != c1Hash {
		t.Errorf("expected HEAD to point to first commit %s, got %s", c1Hash, headHash)
	}

	// Check commit object properties
	c1Bytes, err := r.Store.Get(c1Hash)
	if err != nil {
		t.Fatalf("failed to read first commit: %v", err)
	}
	var c1 objects.Commit
	if err := objects.Decode(c1Bytes, &c1); err != nil {
		t.Fatalf("failed to decode first commit: %v", err)
	}
	if len(c1.Parents) != 0 {
		t.Errorf("expected 0 parents for first commit, got %d", len(c1.Parents))
	}
	if c1.Author.ID != "tester" {
		t.Errorf("expected author tester, got %s", c1.Author.ID)
	}

	// 2. Commit again with no changes -> should return ErrNothingToCommit
	_, err = r.Commit("duplicate commit")
	if !errors.Is(err, ErrNothingToCommit) {
		t.Errorf("expected ErrNothingToCommit, got: %v", err)
	}

	// 3. Stage a file and commit
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("file content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := r.AddFile(filePath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	c2Hash, err := r.Commit("second commit")
	if err != nil {
		t.Fatalf("second commit failed: %v", err)
	}

	// Verify HEAD moved
	headHash, err = refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		t.Fatalf("ResolveHEAD failed: %v", err)
	}
	if headHash != c2Hash {
		t.Errorf("expected HEAD to point to second commit %s, got %s", c2Hash, headHash)
	}

	// Check second commit details
	c2Bytes, err := r.Store.Get(c2Hash)
	if err != nil {
		t.Fatalf("failed to read second commit: %v", err)
	}
	var c2 objects.Commit
	if err := objects.Decode(c2Bytes, &c2); err != nil {
		t.Fatalf("failed to decode second commit: %v", err)
	}
	if len(c2.Parents) != 1 || c2.Parents[0] != c1Hash {
		t.Errorf("expected parent to be %s, got %+v", c1Hash, c2.Parents)
	}

	// 4. Detached HEAD commit
	if err := refs.WriteHEADDetached(r.TwigDir, c1Hash); err != nil {
		t.Fatalf("WriteHEADDetached failed: %v", err)
	}

	// Modify file and stage it
	if err := os.WriteFile(filePath, []byte("updated content"), 0644); err != nil {
		t.Fatalf("failed to update test file: %v", err)
	}
	if err := r.AddFile(filePath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	c3Hash, err := r.Commit("detached commit")
	if err != nil {
		t.Fatalf("detached commit failed: %v", err)
	}

	// Verify HEAD is still detached and resolved to c3Hash
	target, isBranch, err := refs.ReadHEAD(r.TwigDir)
	if err != nil {
		t.Fatalf("ReadHEAD failed: %v", err)
	}
	if isBranch {
		t.Error("expected HEAD to stay detached")
	}
	if target != c3Hash {
		t.Errorf("expected detached HEAD to point directly to commit %s, got %s", c3Hash, target)
	}

	c3Bytes, err := r.Store.Get(c3Hash)
	if err != nil {
		t.Fatalf("failed to read third commit: %v", err)
	}
	var c3 objects.Commit
	if err := objects.Decode(c3Bytes, &c3); err != nil {
		t.Fatalf("failed to decode third commit: %v", err)
	}
	if len(c3.Parents) != 1 || c3.Parents[0] != c1Hash {
		t.Errorf("expected parent of detached commit to be c1 (%s), got %+v", c1Hash, c3.Parents)
	}
}
