package repo

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"twig/internal/metrics"
)

func TestStatusPerfNoRehash(t *testing.T) {
	metrics.Enabled = true
	defer func() { metrics.Enabled = false }()

	tmpDir, err := os.MkdirTemp("", "twig-status-perf-*")
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

	// Setup author config
	configContent := "user.id=perf-test\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Generate 55 files (40 small < 16KB, 15 large >= 16KB)
	rng := rand.New(rand.NewSource(98765))
	var filePaths []string

	// Create small files
	for i := 1; i <= 40; i++ {
		fileName := fmt.Sprintf("small_%d.txt", i)
		filePath := filepath.Join(tmpDir, fileName)
		content := []byte(fmt.Sprintf("content of small file %d", i))
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", fileName, err)
		}
		filePaths = append(filePaths, filePath)
	}

	// Create large files
	for i := 1; i <= 15; i++ {
		fileName := fmt.Sprintf("large_%d.bin", i)
		filePath := filepath.Join(tmpDir, fileName)
		content := make([]byte, 20*1024) // 20KB to force Asset path
		rng.Read(content)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", fileName, err)
		}
		filePaths = append(filePaths, filePath)
	}

	// Add files to staging index
	if err := r.AddFile(tmpDir); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	// Commit staged files
	_, err = r.Commit("Initial commit of 55 files")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Reset HashFileCalls to zero
	metrics.HashFileCalls.Store(0)

	// Call Status() on the unmodified repository
	statusEntries, err := r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Assert that HashFileCalls is zero.
	// We check for a zero call count rather than a fast execution time because wall-clock
	// measurements are subject to environment noise and CPU scheduling, making them flaky.
	// Asserting a zero call count is a deterministic proof that the size/mtime fast-path logic
	// successfully bypassed reading and chunking the file contents.
	callCount := metrics.HashFileCalls.Load()
	if callCount != 0 {
		t.Errorf("expected 0 HashFile calls for unmodified repo, got %d", callCount)
	}

	// Make sure everything is indeed returned as StatusUnmodified (for tracked ones)
	for _, entry := range statusEntries {
		if entry.Path == "config" {
			continue // config file itself is untracked
		}
		if entry.Status != StatusUnmodified {
			t.Errorf("expected %s to be StatusUnmodified, got %v", entry.Path, entry.Status)
		}
	}

	// Modify exactly one file (e.g. small_1.txt)
	small1Path := filepath.Join(tmpDir, "small_1.txt")
	time.Sleep(10 * time.Millisecond) // ensure time changes
	if err := os.WriteFile(small1Path, []byte("content of small file 1 - modified!"), 0644); err != nil {
		t.Fatalf("failed to modify small_1.txt: %v", err)
	}

	// Reset call count again
	metrics.HashFileCalls.Store(0)

	// Call Status() again
	_, err = r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// Assert HashFileCalls is exactly 1 (only the modified file should trigger content rehashing).
	callCount = metrics.HashFileCalls.Load()
	if callCount != 1 {
		t.Errorf("expected exactly 1 HashFile call after modifying one file, got %d", callCount)
	}
}
