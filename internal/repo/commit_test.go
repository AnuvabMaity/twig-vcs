package repo

import (
	"os"
	"testing"
	"time"

	"twig/internal/objects"
	"twig/internal/store"
)

func TestBuildCommitEmptyParents(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-commit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure store layout: %v", err)
	}

	// 1. Build commit with nil parents
	hash, err := BuildCommit(s, "root-tree-hash", nil, "author-1", "initial commit")
	if err != nil {
		t.Fatalf("failed to build commit: %v", err)
	}

	// Retrieve and verify
	data, err := s.Get(hash)
	if err != nil {
		t.Fatalf("failed to get commit from store: %v", err)
	}

	var commit objects.Commit
	if err := objects.Decode(data, &commit); err != nil {
		t.Fatalf("failed to decode commit: %v", err)
	}

	if commit.Type != objects.TypeCommit {
		t.Errorf("expected type %s, got %s", objects.TypeCommit, commit.Type)
	}
	if commit.Root != "root-tree-hash" {
		t.Errorf("expected root tree root-tree-hash, got %s", commit.Root)
	}
	if len(commit.Parents) != 0 {
		t.Errorf("expected 0 parents, got %d", len(commit.Parents))
	}
	if commit.Author.ID != "author-1" {
		t.Errorf("expected author author-1, got %s", commit.Author.ID)
	}
	if commit.Message != "initial commit" {
		t.Errorf("expected msg 'initial commit', got %q", commit.Message)
	}
}

func TestBuildCommitTimeDeterminism(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-commit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("failed to ensure store layout: %v", err)
	}

	// Make two calls in the exact same second
	// Wait until the current second flips to get a full second window
	t0 := time.Now().Unix()
	for time.Now().Unix() == t0 {
		time.Sleep(5 * time.Millisecond)
	}

	hash1, err := BuildCommit(s, "tree-hash", []string{"parent1"}, "committer", "test commit")
	if err != nil {
		t.Fatalf("failed build 1: %v", err)
	}

	hash2, err := BuildCommit(s, "tree-hash", []string{"parent1"}, "committer", "test commit")
	if err != nil {
		t.Fatalf("failed build 2: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected identical hashes within same second, got %s and %s", hash1, hash2)
	}

	// Now wait until the second boundary flips
	t1 := time.Now().Unix()
	for time.Now().Unix() == t1 {
		time.Sleep(5 * time.Millisecond)
	}

	hash3, err := BuildCommit(s, "tree-hash", []string{"parent1"}, "committer", "test commit")
	if err != nil {
		t.Fatalf("failed build 3: %v", err)
	}

	if hash1 == hash3 {
		t.Errorf("expected different hashes across seconds, but got identical hash: %s", hash1)
	}
}
