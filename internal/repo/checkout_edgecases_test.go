package repo

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/objects"
)

// TestCheckoutEmptyCommit checks that checking out a commit whose tree has
// zero entries succeeds and leaves the working directory empty (aside from .twig).
func TestCheckoutEmptyCommit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-edgecase-empty-*")
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

	// Commit 1: Empty commit (no files staged)
	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	cfg := map[string]string{"user.id": "edgecase-tester"}
	if err := objects.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	emptyCommitHash, err := r.Commit("empty initial commit")
	if err != nil {
		t.Fatalf("empty commit failed: %v", err)
	}

	// Commit 2: Write and commit a file
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := r.AddFile(filePath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	if _, err := r.Commit("commit with file"); err != nil {
		t.Fatalf("commit 2 failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("file should exist: %v", err)
	}

	// Checkout Commit 1 (the empty commit)
	if err := r.Checkout(emptyCommitHash, false); err != nil {
		t.Fatalf("Checkout empty commit failed: %v", err)
	}

	// Verify working directory is empty (except .twig)
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read working dir: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() != objects.DefaultTwigDir {
			t.Errorf("expected working directory to be empty (except .twig), found: %s", entry.Name())
		}
	}
}

// TestCheckoutForceOverwrite checks the force overwrite path, verifying
// that modifications are rejected by default but overwritten with force=true.
func TestCheckoutForceOverwrite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-edgecase-force-*")
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

	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	cfg := map[string]string{"user.id": "edgecase-tester"}
	if err := objects.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Commit 1: Write and commit a file
	filePath := filepath.Join(tmpDir, "a.txt")
	initialContent := []byte("initial content")
	if err := os.WriteFile(filePath, initialContent, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := r.AddFile(filePath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	c1Hash, err := r.Commit("first commit")
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// Locally modify the file in working directory
	editedContent := []byte("edited local content")
	if err := os.WriteFile(filePath, editedContent, 0644); err != nil {
		t.Fatalf("failed to edit file: %v", err)
	}

	// Checkout without force should fail
	err = r.Checkout(c1Hash, false)
	if !errors.Is(err, ErrWouldOverwriteChanges) {
		t.Errorf("expected error %v, got %v", ErrWouldOverwriteChanges, err)
	}

	// Verify content remains the edited content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !bytes.Equal(content, editedContent) {
		t.Errorf("expected content to be the local edit, got %q", content)
	}

	// Checkout with force should succeed
	if err := r.Checkout(c1Hash, true); err != nil {
		t.Fatalf("forced checkout failed: %v", err)
	}

	// Verify content is restored to the committed content
	content, err = os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !bytes.Equal(content, initialContent) {
		t.Errorf("expected content to be restored to %q, got %q", initialContent, content)
	}
}

// TestCheckoutSameRefTwice checks that checking out the same ref twice in a row
// succeeds without error and leaves the working directory unchanged.
func TestCheckoutSameRefTwice(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-edgecase-sameref-*")
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

	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	cfg := map[string]string{"user.id": "edgecase-tester"}
	if err := objects.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	filePath := filepath.Join(tmpDir, "a.txt")
	contentVal := []byte("stable content")
	if err := os.WriteFile(filePath, contentVal, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := r.AddFile(filePath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	cHash, err := r.Commit("commit")
	if err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	// 1. First checkout
	if err := r.Checkout(cHash, false); err != nil {
		t.Fatalf("first checkout failed: %v", err)
	}

	// 2. Second checkout (same ref)
	if err := r.Checkout(cHash, false); err != nil {
		t.Fatalf("second checkout failed: %v", err)
	}

	// Verify working directory state is intact
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if !bytes.Equal(content, contentVal) {
		t.Errorf("content changed during no-op checkout")
	}
}
