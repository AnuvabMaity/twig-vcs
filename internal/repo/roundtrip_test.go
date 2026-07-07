package repo

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/objects"
)

func TestRoundTripIntegration(t *testing.T) {
	// 1. Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "twig-roundtrip-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Initialize repo
	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// 3. Write mock config for committer identification
	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	cfg := map[string]string{"user.id": "roundtrip-tester"}
	if err := objects.WriteConfig(configPath, cfg); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// 4. Create source fixture files
	// - a.txt: under 16KB (Blob)
	// - nested/b.txt: under 16KB (Blob)
	// - nested/deep/large.bin: 1.2MB (Asset)
	aContent := []byte("This is the first simple blob file.")
	bContent := []byte("This is the second simple blob file in a nested folder.")

	largeSize := 1200 * 1024 // 1.2MB
	largeContent := make([]byte, largeSize)
	rnd := rand.New(rand.NewSource(42))
	rnd.Read(largeContent)

	aPath := filepath.Join(tmpDir, "a.txt")
	nestedDir := filepath.Join(tmpDir, "nested")
	deepDir := filepath.Join(nestedDir, "deep")
	bPath := filepath.Join(nestedDir, "b.txt")
	largePath := filepath.Join(deepDir, "large.bin")

	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("failed to create nested directories: %v", err)
	}

	if err := os.WriteFile(aPath, aContent, 0644); err != nil {
		t.Fatalf("failed to write a.txt: %v", err)
	}
	if err := os.WriteFile(bPath, bContent, 0644); err != nil {
		t.Fatalf("failed to write b.txt: %v", err)
	}
	if err := os.WriteFile(largePath, largeContent, 0644); err != nil {
		t.Fatalf("failed to write large.bin: %v", err)
	}

	// 5. Add and commit all files in the working directory
	if err := r.AddFile(tmpDir); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	commitHash, err := r.Commit("initial roundtrip commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 6. Delete all files in working directory except .twig
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read working dir: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() == objects.DefaultTwigDir {
			continue
		}
		pathToDelete := filepath.Join(tmpDir, entry.Name())
		if err := os.RemoveAll(pathToDelete); err != nil {
			t.Fatalf("failed to delete working directory item %s: %v", entry.Name(), err)
		}
	}

	// 7. Perform Checkout to restore files
	// We use force = false because working directory is empty and we want to verify
	// the standard checkout flow without overwrite warnings.
	if err := r.Checkout(commitHash, false); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// 8. Walk the restored directory and compare content byte-for-byte
	// We choose full byte-for-byte comparison to absolutely guarantee lossless restoration
	// of files including chunk reconstruction for multi-chunk Asset files.
	reconA, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("failed to read restored a.txt: %v", err)
	}
	if !bytes.Equal(reconA, aContent) {
		t.Errorf("restored a.txt content mismatch")
	}

	reconB, err := os.ReadFile(bPath)
	if err != nil {
		t.Fatalf("failed to read restored b.txt: %v", err)
	}
	if !bytes.Equal(reconB, bContent) {
		t.Errorf("restored b.txt content mismatch")
	}

	reconLarge, err := os.ReadFile(largePath)
	if err != nil {
		t.Fatalf("failed to read restored large.bin: %v", err)
	}
	if !bytes.Equal(reconLarge, largeContent) {
		t.Errorf("restored large.bin content mismatch")
	}
}
