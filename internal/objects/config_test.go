package objects

import (
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config")
	expected := map[string]string{
		"user.name": "Alice Liddell",
		"user.id":   "alice",
		"core.mode": "loose",
	}

	err = WriteConfig(configPath, expected)
	if err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	actual, err := ReadConfig(configPath)
	if err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

func TestResolveAuthorID_WithConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config")
	configData := map[string]string{
		"user.id": "alice",
	}

	err = WriteConfig(configPath, configData)
	if err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	authorID, err := ResolveAuthorID(configPath)
	if err != nil {
		t.Fatalf("ResolveAuthorID failed: %v", err)
	}

	if authorID != "alice" {
		t.Errorf("Expected author ID 'alice', got %q", authorID)
	}
}

func TestResolveAuthorID_Fallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Use a path to a non-existent file to force fallback
	configPath := filepath.Join(tmpDir, "nonexistent-config")

	currUser, err := user.Current()
	if err != nil {
		t.Skipf("Skipping fallback test as user.Current() failed: %v", err)
	}
	expectedUsername := currUser.Username

	authorID, err := ResolveAuthorID(configPath)
	if err != nil {
		t.Fatalf("ResolveAuthorID failed: %v", err)
	}

	if authorID != expectedUsername {
		t.Errorf("Expected fallback author ID %q, got %q", expectedUsername, authorID)
	}
}
