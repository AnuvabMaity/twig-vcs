package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"twig/internal/refs"
)

func TestRepoCreateBranch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-repo-branch-test-*")
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
	// 1. Create branch on unborn branch should fail
	err = r.CreateBranch("feature")
	if !errors.Is(err, refs.ErrUnbornBranch) {
		t.Errorf("expected ErrUnbornBranch, got: %v", err)
	}
	// 2. Commit a file to main so we have a commit to branch from
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := r.AddFile(filePath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	commitHash, err := r.Commit("Commit 1")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	// 3. Create branch should succeed now
	if err := r.CreateBranch("feature"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	// Read new branch ref and verify it points to commitHash
	refHash, err := refs.ReadBranch(r.TwigDir, "feature")
	if err != nil {
		t.Fatalf("ReadBranch failed for feature branch: %v", err)
	}
	if refHash != commitHash {
		t.Errorf("expected feature branch to point to %s, got %s", commitHash, refHash)
	}
	// 4. Creating the same branch again should return ErrBranchExists
	err = r.CreateBranch("feature")
	if !errors.Is(err, ErrBranchExists) {
		t.Errorf("expected ErrBranchExists, got: %v", err)
	}
}
