package refs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRefsLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-refs-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up basic directories that twig init would create
	twigDir := filepath.Join(tmpDir, ".twig")
	if err := os.MkdirAll(filepath.Join(twigDir, refsDirName, headsDirName), 0755); err != nil {
		t.Fatalf("failed to create refs dir: %v", err)
	}

	// Write mock HEAD pointing symbolically to main (as twig init does)
	headPath := filepath.Join(twigDir, headFileName)
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatalf("failed to create mock HEAD: %v", err)
	}

	// 1. Fresh repo checks
	target, isBranch, err := ReadHEAD(twigDir)
	if err != nil {
		t.Fatalf("ReadHEAD failed: %v", err)
	}
	if !isBranch || target != "main" {
		t.Errorf("expected isBranch=true and target='main', got isBranch=%t and target=%q", isBranch, target)
	}

	_, err = ResolveHEAD(twigDir)
	if !errors.Is(err, ErrUnbornBranch) {
		t.Errorf("expected ErrUnbornBranch, got %v", err)
	}

	// 2. ReadBranch on non-existent branch
	_, err = ReadBranch(twigDir, "main")
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error, got: %v", err)
	}

	// 3. Write branch and ResolveHEAD
	commitHash := "abc123xyz789"
	if err := WriteBranch(twigDir, "main", commitHash); err != nil {
		t.Fatalf("WriteBranch failed: %v", err)
	}

	resolved, err := ResolveHEAD(twigDir)
	if err != nil {
		t.Fatalf("ResolveHEAD failed after write branch: %v", err)
	}
	if resolved != commitHash {
		t.Errorf("expected resolved commit hash %q, got %q", commitHash, resolved)
	}

	readVal, err := ReadBranch(twigDir, "main")
	if err != nil {
		t.Fatalf("ReadBranch failed: %v", err)
	}
	if readVal != commitHash {
		t.Errorf("expected read branch value %q, got %q", commitHash, readVal)
	}

	// 4. WriteHEADDetached and ReadHEAD
	detachedHash := "999888777666"
	if err := WriteHEADDetached(twigDir, detachedHash); err != nil {
		t.Fatalf("WriteHEADDetached failed: %v", err)
	}

	target, isBranch, err = ReadHEAD(twigDir)
	if err != nil {
		t.Fatalf("ReadHEAD failed: %v", err)
	}
	if isBranch {
		t.Error("expected detached HEAD to not be a branch")
	}
	if target != detachedHash {
		t.Errorf("expected target %q, got %q", detachedHash, target)
	}

	resolved, err = ResolveHEAD(twigDir)
	if err != nil {
		t.Fatalf("ResolveHEAD failed on detached HEAD: %v", err)
	}
	if resolved != detachedHash {
		t.Errorf("expected resolved detached HEAD commit %q, got %q", detachedHash, resolved)
	}

	// 5. WriteHEAD symbolic switch
	if err := WriteHEAD(twigDir, "feature-branch"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}

	target, isBranch, err = ReadHEAD(twigDir)
	if err != nil {
		t.Fatalf("ReadHEAD failed: %v", err)
	}
	if !isBranch || target != "feature-branch" {
		t.Errorf("expected symbolic switch to feature-branch, got isBranch=%t target=%q", isBranch, target)
	}
}

func TestListBranches(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-refs-list-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	twigDir := filepath.Join(tmpDir, ".twig")
	if err := os.MkdirAll(filepath.Join(twigDir, refsDirName, headsDirName), 0755); err != nil {
		t.Fatalf("failed to create refs dir: %v", err)
	}

	// 1. Initially empty listing
	branches, err := ListBranches(twigDir)
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("expected 0 branches, got %d", len(branches))
	}

	// 2. Create branches
	if err := WriteBranch(twigDir, "main", "hash1"); err != nil {
		t.Fatalf("failed to write main branch: %v", err)
	}
	if err := WriteBranch(twigDir, "feature/abc", "hash2"); err != nil {
		t.Fatalf("failed to write nested branch: %v", err)
	}

	branches, err = ListBranches(twigDir)
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}

	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(branches), branches)
	}

	hasMain := false
	hasNested := false
	for _, b := range branches {
		if b == "main" {
			hasMain = true
		}
		if b == "feature/abc" {
			hasNested = true
		}
	}

	if !hasMain {
		t.Error("expected list to contain 'main'")
	}
	if !hasNested {
		t.Error("expected list to contain 'feature/abc'")
	}
}

