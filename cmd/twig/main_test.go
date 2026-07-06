package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"twig/internal/hashing"
	"twig/internal/objects"
)

// TestCLISkeleton compiles the twig binary and tests subcommand dispatching.
func TestCLISkeleton(t *testing.T) {
	// Create a temporary directory for building the twig binary
	tmpDir, err := os.MkdirTemp("", "twig-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "twig")
	if os.PathSeparator == '\\' {
		binaryPath += ".exe"
	}

	// Build the twig binary
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build twig binary: %v", err)
	}

	tests := []struct {
		name           string
		args           []string
		expectedExit   int
		expectedStdout string
		expectedStderr string
	}{
		{
			name:           "No arguments",
			args:           []string{},
			expectedExit:   1,
			expectedStdout: "",
			expectedStderr: "Usage: twig <command> [<args>]",
		},
		{
			name:           "Bogus subcommand",
			args:           []string{"bogus"},
			expectedExit:   1,
			expectedStdout: "",
			expectedStderr: "Usage: twig <command> [<args>]",
		},
		{
			name:           "init command",
			args:           []string{"init"},
			expectedExit:   0,
			expectedStdout: "init: not implemented",
			expectedStderr: "",
		},
		{
			name:           "add command",
			args:           []string{"add"},
			expectedExit:   0,
			expectedStdout: "add: not implemented",
			expectedStderr: "",
		},
		{
			name:           "commit command",
			args:           []string{"commit"},
			expectedExit:   0,
			expectedStdout: "commit: not implemented",
			expectedStderr: "",
		},
		{
			name:           "log command",
			args:           []string{"log"},
			expectedExit:   0,
			expectedStdout: "log: not implemented",
			expectedStderr: "",
		},
		{
			name:           "checkout command",
			args:           []string{"checkout"},
			expectedExit:   0,
			expectedStdout: "checkout: not implemented",
			expectedStderr: "",
		},
		{
			name:           "status command",
			args:           []string{"status"},
			expectedExit:   0,
			expectedStdout: "status: not implemented",
			expectedStderr: "",
		},
		{
			name:           "branch command",
			args:           []string{"branch"},
			expectedExit:   0,
			expectedStdout: "branch: not implemented",
			expectedStderr: "",
		},
		{
			name:           "hash-object command missing file",
			args:           []string{"hash-object"},
			expectedExit:   1,
			expectedStdout: "",
			expectedStderr: "Usage: twig hash-object [--store <dir>] <file>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			exitCode := 0
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode = exitError.ExitCode()
				} else {
					t.Fatalf("Failed to run binary: %v", err)
				}
			}

			if exitCode != tc.expectedExit {
				t.Errorf("Expected exit code %d, got %d", tc.expectedExit, exitCode)
			}

			stdoutStr := strings.TrimSpace(stdout.String())
			if tc.expectedStdout != "" {
				if !strings.Contains(stdoutStr, tc.expectedStdout) {
					t.Errorf("Expected stdout to contain %q, got %q", tc.expectedStdout, stdoutStr)
				}
			} else if stdoutStr != "" {
				t.Errorf("Expected empty stdout, got %q", stdoutStr)
			}

			stderrStr := strings.TrimSpace(stderr.String())
			if tc.expectedStderr != "" {
				if !strings.Contains(stderrStr, tc.expectedStderr) {
					t.Errorf("Expected stderr to contain %q, got %q", tc.expectedStderr, stderrStr)
				}
			} else if stderrStr != "" {
				t.Errorf("Expected empty stderr, got %q", stderrStr)
			}
		})
	}
}

// TestCLIHashObject verifies the end-to-end functionality of twig hash-object.
func TestCLIHashObject(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "twig-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath := filepath.Join(tmpDir, "twig")
	if os.PathSeparator == '\\' {
		binaryPath += ".exe"
	}

	// Build the twig binary
	buildCmd := exec.Command("go", "build", "-o", binaryPath)
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build twig binary: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(tmpDir, "hello.txt")
	content := []byte("hello, this is a test file for the hash-object command!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Store path
	storePath := filepath.Join(tmpDir, ".twig")

	// Compute expected hash manually using objects and hashing wrapper
	blob := objects.Blob{
		Type: objects.TypeBlob,
		Data: content,
	}
	encoded, err := objects.Encode(blob)
	if err != nil {
		t.Fatalf("Failed to encode blob: %v", err)
	}
	expectedHash := hashing.Hash(encoded)

	// Run twig hash-object
	cmd := exec.Command(binaryPath, "hash-object", "--store", storePath, testFile)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run hash-object: %v. Stderr: %q", err, stderr.String())
	}

	actualHash := strings.TrimSpace(stdout.String())
	if actualHash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, actualHash)
	}

	// Verify that the object file is written to the store
	objectFile := hashing.ObjectPath(storePath, expectedHash)
	if _, err := os.Stat(objectFile); err != nil {
		t.Errorf("Expected object file to exist at %s, but got error: %v", objectFile, err)
	}

	// Verify running it twice is deduplicated (still one file)
	cmd2 := exec.Command(binaryPath, "hash-object", "--store", storePath, testFile)
	if err := cmd2.Run(); err != nil {
		t.Fatalf("Failed to run hash-object second time: %v", err)
	}

	// Count files under storePath/objects
	objectsDir := filepath.Join(storePath, "objects")
	fileCount := 0
	err = filepath.WalkDir(objectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	if fileCount != 1 {
		t.Errorf("Expected exactly 1 object file in store, found %d", fileCount)
	}

	// Verify running it on non-existent file returns non-zero code
	cmdErr := exec.Command(binaryPath, "hash-object", "--store", storePath, filepath.Join(tmpDir, "nonexistent.txt"))
	if err := cmdErr.Run(); err == nil {
		t.Error("Expected error when running hash-object on non-existent file, got nil")
	}
}
