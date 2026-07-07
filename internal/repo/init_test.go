package repo

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"twig/internal/index"
	"twig/internal/objects"
)

func TestInit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-init-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize the repo
	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	twigDir := filepath.Join(tmpDir, ".twig")

	// 1. Verify on-disk directories
	paths := []string{
		filepath.Join(twigDir, "objects"),
		filepath.Join(twigDir, "refs", "heads"),
		filepath.Join(twigDir, "HEAD"),
		filepath.Join(twigDir, "index"),
		filepath.Join(twigDir, "config"),
		filepath.Join(twigDir, "VERSION"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected path %s to exist: %v", path, err)
		}
	}

	// 2. Verify HEAD content
	headBytes, err := os.ReadFile(filepath.Join(twigDir, "HEAD"))
	if err != nil {
		t.Fatalf("failed to read HEAD: %v", err)
	}
	expectedHEAD := "ref: refs/heads/main\n"
	if string(headBytes) != expectedHEAD {
		t.Errorf("expected HEAD %q, got %q", expectedHEAD, string(headBytes))
	}

	// 3. Verify VERSION content
	versionBytes, err := os.ReadFile(filepath.Join(twigDir, "VERSION"))
	if err != nil {
		t.Fatalf("failed to read VERSION: %v", err)
	}
	expectedVersion := strconv.Itoa(objects.FormatVersion) + "\n"
	if string(versionBytes) != expectedVersion {
		t.Errorf("expected VERSION %q, got %q", expectedVersion, string(versionBytes))
	}

	// 4. Verify index is readable and empty
	idx, err := index.Load(filepath.Join(twigDir, "index"))
	if err != nil {
		t.Fatalf("failed to load initialized index: %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("expected empty index entries, got %d", len(idx.Entries))
	}

	// 5. Verify config is readable and empty
	cfg, err := objects.ReadConfig(filepath.Join(twigDir, "config"))
	if err != nil {
		t.Fatalf("failed to read initialized config: %v", err)
	}
	if len(cfg) != 0 {
		t.Errorf("expected empty config entries, got %d", len(cfg))
	}

	// 6. Init again in the same directory should fail
	err = Init(tmpDir)
	if !errors.Is(err, ErrRepoExists) {
		t.Errorf("expected ErrRepoExists on second init, got: %v", err)
	}
}
