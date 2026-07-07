package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	// Create a temp directory for the test repo structures
	tmpDir, err := os.MkdirTemp("", "twig-repo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Sub-folders:
	// tmpDir/repo1/ - has .twig
	// tmpDir/repo1/src/pkg/ - nested directory
	// tmpDir/nonrepo/ - no .twig
	repo1 := filepath.Join(tmpDir, "repo1")
	repo1Twig := filepath.Join(repo1, ".twig")
	repo1Nested := filepath.Join(repo1, "src", "pkg")
	nonRepo := filepath.Join(tmpDir, "nonrepo")

	if err := os.MkdirAll(repo1Twig, 0755); err != nil {
		t.Fatalf("failed to create repo1 .twig: %v", err)
	}
	if err := os.MkdirAll(repo1Nested, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}
	if err := os.MkdirAll(nonRepo, 0755); err != nil {
		t.Fatalf("failed to create nonrepo dir: %v", err)
	}

	// 1. Called from repo root
	root, twig, err := FindRoot(repo1)
	if err != nil {
		t.Errorf("FindRoot from repo root failed: %v", err)
	}
	if root != repo1 || twig != repo1Twig {
		t.Errorf("expected (%q, %q), got (%q, %q)", repo1, repo1Twig, root, twig)
	}

	// 2. Called from deep nested subdirectory
	root, twig, err = FindRoot(repo1Nested)
	if err != nil {
		t.Errorf("FindRoot from nested dir failed: %v", err)
	}
	if root != repo1 || twig != repo1Twig {
		t.Errorf("expected (%q, %q), got (%q, %q)", repo1, repo1Twig, root, twig)
	}

	// 3. Called from non-repo directory
	_, _, err = FindRoot(nonRepo)
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("expected ErrNotARepo, got: %v", err)
	}
}
