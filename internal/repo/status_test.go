package repo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"twig/internal/index"
)

func TestRepoStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-status-test-*")
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

	// Helper to find a status entry in the list
	getStatus := func(entries []StatusEntry, path string) []FileStatus {
		var stats []FileStatus
		for _, e := range entries {
			if e.Path == path {
				stats = append(stats, e.Status)
			}
		}
		return stats
	}

	// 1. Untracked file
	untrackedPath := filepath.Join(tmpDir, "untracked.txt")
	if err := os.WriteFile(untrackedPath, []byte("untracked file content"), 0644); err != nil {
		t.Fatalf("failed to write untracked.txt: %v", err)
	}

	statusEntries, err := r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	stats := getStatus(statusEntries, "untracked.txt")
	if len(stats) != 1 || stats[0] != StatusUntracked {
		t.Errorf("expected untracked.txt to be StatusUntracked, got %v", stats)
	}

	// 2. Staged New file
	stagedNewPath := filepath.Join(tmpDir, "staged_new.txt")
	if err := os.WriteFile(stagedNewPath, []byte("staged new content"), 0644); err != nil {
		t.Fatalf("failed to write staged_new.txt: %v", err)
	}

	if err := r.AddFile(stagedNewPath); err != nil {
		t.Fatalf("AddFile failed for staged_new.txt: %v", err)
	}

	statusEntries, err = r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	stats = getStatus(statusEntries, "staged_new.txt")
	if len(stats) != 1 || stats[0] != StatusStagedNew {
		t.Errorf("expected staged_new.txt to be StatusStagedNew, got %v", stats)
	}

	// 3. Commit staged files, check Unmodified
	// Write dummy author identity config so commit works
	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err = r.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	statusEntries, err = r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	stats = getStatus(statusEntries, "staged_new.txt")
	if len(stats) != 1 || stats[0] != StatusUnmodified {
		t.Errorf("expected staged_new.txt to be StatusUnmodified after commit, got %v", stats)
	}

	// 4. Modified file (working directory change)
	// We modify staged_new.txt's content and ensure mtime updates
	time.Sleep(10 * time.Millisecond) // ensure time changes
	if err := os.WriteFile(stagedNewPath, []byte("staged new content modified"), 0644); err != nil {
		t.Fatalf("failed to modify staged_new.txt: %v", err)
	}

	statusEntries, err = r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	stats = getStatus(statusEntries, "staged_new.txt")
	if len(stats) != 1 || stats[0] != StatusModified {
		t.Errorf("expected staged_new.txt to be StatusModified, got %v", stats)
	}

	// 5. Staged Modified file (stage the change)
	if err := r.AddFile(stagedNewPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	statusEntries, err = r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	stats = getStatus(statusEntries, "staged_new.txt")
	if len(stats) != 1 || stats[0] != StatusStagedModified {
		t.Errorf("expected staged_new.txt to be StatusStagedModified, got %v", stats)
	}

	// 6. Double appearance: staged modified AND modified in working directory again
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(stagedNewPath, []byte("staged new content modified twice"), 0644); err != nil {
		t.Fatalf("failed to modify staged_new.txt again: %v", err)
	}

	statusEntries, err = r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	stats = getStatus(statusEntries, "staged_new.txt")
	if len(stats) != 2 {
		t.Errorf("expected staged_new.txt to have 2 statuses, got %v", stats)
	}
	hasStagedMod := false
	hasMod := false
	for _, st := range stats {
		if st == StatusStagedModified {
			hasStagedMod = true
		}
		if st == StatusModified {
			hasMod = true
		}
	}
	if !hasStagedMod || !hasMod {
		t.Errorf("expected both StatusStagedModified and StatusModified for staged_new.txt, got %v", stats)
	}

	// 7. Deleted file
	deletedPath := filepath.Join(tmpDir, "deleted.txt")
	if err := os.WriteFile(deletedPath, []byte("to be deleted"), 0644); err != nil {
		t.Fatalf("failed to write deleted.txt: %v", err)
	}
	if err := r.AddFile(deletedPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	// Delete from disk
	if err := os.Remove(deletedPath); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	statusEntries, err = r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	stats = getStatus(statusEntries, "deleted.txt")
	hasDeleted := false
	for _, st := range stats {
		if st == StatusDeleted {
			hasDeleted = true
		}
	}
	if !hasDeleted {
		t.Errorf("expected deleted.txt to have StatusDeleted, got %v", stats)
	}
}

func TestStatusWithUnbornBranch(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-status-unborn-*")
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

	// Add file on unborn branch -> Should be staged-new
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	if err := r.AddFile(filePath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	entries, err := r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Path == "file.txt" {
			found = true
			if e.Status != StatusStagedNew {
				t.Errorf("expected StatusStagedNew on unborn branch, got %v", e.Status)
			}
		}
	}
	if !found {
		t.Error("file.txt not found in status entries")
	}
}

// TestStatusIndexMissing does NeedsRehash and checks loading index when missing
func TestStatusMissingIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-status-missing-index-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Delete index file
	indexPath := filepath.Join(tmpDir, ".twig", "index")
	if err := os.Remove(indexPath); err != nil {
		t.Fatalf("failed to remove index file: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Create file on disk
	filePath := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	entries, err := r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Empty index: file.txt should be untracked
	found := false
	for _, e := range entries {
		if e.Path == "file.txt" {
			found = true
			if e.Status != StatusUntracked {
				t.Errorf("expected untracked for file.txt, got %v", e.Status)
			}
		}
	}
	if !found {
		t.Error("expected file.txt to be untracked")
	}
}

func TestIndexNeedsRehashErrors(t *testing.T) {
	// Make sure NeedsRehash returns error when stat fails on non-existent file
	_, err := index.NeedsRehash("/nonexistent/path/here", index.Entry{})
	if err == nil {
		t.Error("expected NeedsRehash to fail on nonexistent path")
	}
}
