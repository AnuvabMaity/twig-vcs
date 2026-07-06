package main

import (
	"bytes"
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
			name:           "hash-object command",
			args:           []string{"hash-object"},
			expectedExit:   0,
			expectedStdout: "hash-object: not implemented",
			expectedStderr: "",
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
