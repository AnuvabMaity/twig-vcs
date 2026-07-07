package main

import (
	"bytes"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
			expectedStdout: "Initialized empty Twig repository in ./.twig/",
			expectedStderr: "",
		},
		{
			name:           "add command",
			args:           []string{"add"},
			expectedExit:   1,
			expectedStdout: "",
			expectedStderr: "Usage: twig add <path> [<path>...]",
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
		{
			name:           "cat-object command missing arguments",
			args:           []string{"cat-object"},
			expectedExit:   1,
			expectedStdout: "",
			expectedStderr: "Usage: twig cat-object [--store <dir>] <hash> <type>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...)
			cmd.Dir = tmpDir
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

// TestCLIRoundTrips verifies both Blob and Asset paths end-to-end via CLI.
func TestCLIRoundTrips(t *testing.T) {
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

	storePath := filepath.Join(tmpDir, ".twig")

	// 1. Small file (Blob path)
	smallFile := filepath.Join(tmpDir, "small.txt")
	smallContent := []byte("hello world! this is a small file content to test blob path.")
	if err := os.WriteFile(smallFile, smallContent, 0644); err != nil {
		t.Fatalf("Failed to create small test file: %v", err)
	}

	// Run hash-object
	cmdSmallHash := exec.Command(binaryPath, "hash-object", "--store", storePath, smallFile)
	var stdoutSmallHash, stderrSmallHash bytes.Buffer
	cmdSmallHash.Stdout = &stdoutSmallHash
	cmdSmallHash.Stderr = &stderrSmallHash
	if err := cmdSmallHash.Run(); err != nil {
		t.Fatalf("hash-object failed: %v. Stderr: %q", err, stderrSmallHash.String())
	}
	smallHash := strings.TrimSpace(stdoutSmallHash.String())

	// Run cat-object
	cmdSmallCat := exec.Command(binaryPath, "cat-object", "--store", storePath, smallHash, "blob")
	var stdoutSmallCat, stderrSmallCat bytes.Buffer
	cmdSmallCat.Stdout = &stdoutSmallCat
	cmdSmallCat.Stderr = &stderrSmallCat
	if err := cmdSmallCat.Run(); err != nil {
		t.Fatalf("cat-object failed: %v. Stderr: %q", err, stderrSmallCat.String())
	}
	if !bytes.Equal(smallContent, stdoutSmallCat.Bytes()) {
		t.Errorf("Reconstructed small file content does not match. Expected %q, got %q", smallContent, stdoutSmallCat.Bytes())
	}

	// 2. Large file (Asset path)
	largeFile := filepath.Join(tmpDir, "large.txt")
	largeSize := 1536 * 1024 // 1.5 MB
	largeContent := make([]byte, largeSize)
	r := rand.New(rand.NewSource(12345))
	if _, err := r.Read(largeContent); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}
	if err := os.WriteFile(largeFile, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	// Run hash-object
	cmdLargeHash := exec.Command(binaryPath, "hash-object", "--store", storePath, largeFile)
	var stdoutLargeHash, stderrLargeHash bytes.Buffer
	cmdLargeHash.Stdout = &stdoutLargeHash
	cmdLargeHash.Stderr = &stderrLargeHash
	if err := cmdLargeHash.Run(); err != nil {
		t.Fatalf("hash-object failed: %v. Stderr: %q", err, stderrLargeHash.String())
	}
	largeHash := strings.TrimSpace(stdoutLargeHash.String())

	// Run cat-object
	cmdLargeCat := exec.Command(binaryPath, "cat-object", "--store", storePath, largeHash, "asset")
	var stdoutLargeCat, stderrLargeCat bytes.Buffer
	cmdLargeCat.Stdout = &stdoutLargeCat
	cmdLargeCat.Stderr = &stderrLargeCat
	if err := cmdLargeCat.Run(); err != nil {
		t.Fatalf("cat-object failed: %v. Stderr: %q", err, stderrLargeCat.String())
	}
	if !bytes.Equal(largeContent, stdoutLargeCat.Bytes()) {
		t.Errorf("Reconstructed large file content does not match original")
	}

	// 3. Error Case: Invalid Type
	cmdInvalid := exec.Command(binaryPath, "cat-object", "--store", storePath, smallHash, "invalidtype")
	if err := cmdInvalid.Run(); err == nil {
		t.Error("Expected error when calling cat-object with invalid type, got nil")
	}
}
