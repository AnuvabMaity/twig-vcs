package hashing

import (
	"path/filepath"
	"testing"
)

func TestHashDeterminism(t *testing.T) {
	input := []byte("hello world")
	expected := Hash(input)

	for range 10 {
		actual := Hash(input)
		if actual != expected {
			t.Errorf("Hash is not deterministic. Expected: %s, got: %s", expected, actual)
		}
	}

	// Verify length (32 bytes = 64 hex characters)
	if len(expected) != 64 {
		t.Errorf("Expected 64 character hex string, got %d", len(expected))
	}
}

func TestHashDistinctness(t *testing.T) {
	inputs := [][]byte{
		[]byte("hello"),
		[]byte("world"),
		[]byte("twig"),
	}

	hashes := make(map[string]bool)
	for _, in := range inputs {
		h := Hash(in)
		if hashes[h] {
			t.Errorf("Duplicate hash found: %s", h)
		}
		hashes[h] = true
	}

	if len(hashes) != 3 {
		t.Errorf("Expected 3 unique hashes, got %d", len(hashes))
	}
}

func TestObjectPath(t *testing.T) {
	twigDir := filepath.Join("repo", ".twig")
	hash := "abcd1234567890abcdef"
	expected := filepath.Join(twigDir, "objects", "ab", "cd1234567890abcdef")

	actual := ObjectPath(twigDir, hash)
	if actual != expected {
		t.Errorf("ObjectPath failed. Expected: %s, got: %s", expected, actual)
	}
}
