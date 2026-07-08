package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"twig/internal/compress"
	"twig/internal/hashing"
	"twig/internal/metrics"
	"twig/internal/objects"
)

// Store represents a loose object, content-addressable key-value store.
type Store struct {
	twigDir string
}

// Open returns a Store rooted at twigDir (the .twig directory).
// It does not require twigDir to already exist; call EnsureLayout first if needed.
func Open(twigDir string) *Store {
	return &Store{
		twigDir: twigDir,
	}
}

// EnsureLayout creates the objects/ directory (and any other required
// subdirectories) under twigDir if they don't already exist.
func (s *Store) EnsureLayout() error {
	objectsDir := filepath.Join(s.twigDir, objects.ObjectsDirName)
	if err := os.MkdirAll(objectsDir, objects.DirPermMode); err != nil {
		return fmt.Errorf("failed to create objects directory: %w", err)
	}
	return nil
}

// Has reports whether an object with this hash already exists.
func (s *Store) Has(hash string) (bool, error) {
	path := hashing.ObjectPath(s.twigDir, hash)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check existence of object %s: %w", hash, err)
}

// Put computes the hash of content, and if no object with that hash
// already exists, compresses and writes it to disk. If it already exists,
// Put does nothing further (this is the dedup point). Either way, it
// returns the hash.
func (s *Store) Put(content []byte) (string, error) {
	if metrics.Enabled {
		metrics.StorePutCalls.Add(1)
	}
	hash := hashing.Hash(content)
	exists, err := s.Has(hash)
	if err != nil {
		return "", err
	}
	if exists {
		if metrics.Enabled {
			metrics.StorePutDedupSkips.Add(1)
		}
		return hash, nil
	}

	// Compress the content
	compressed, err := compress.Compress(content)
	if err != nil {
		return "", fmt.Errorf("failed to compress content: %w", err)
	}

	path := hashing.ObjectPath(s.twigDir, hash)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create fan-out directory %s: %w", dir, err)
	}

	// Write atomically: write to a temp file in the same directory and rename
	tempFile, err := os.CreateTemp(dir, "put-tmp-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempName := tempFile.Name()
	var tempClosed bool
	var success bool
	defer func() {
		// Clean up the temp file if we fail before renaming it successfully
		if !success {
			if !tempClosed {
				tempFile.Close()
			}
			os.Remove(tempName)
		}
	}()

	if _, err := tempFile.Write(compressed); err != nil {
		return "", fmt.Errorf("failed to write compressed content to temp file: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return "", fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}
	tempClosed = true // prevent defer cleanup from trying to close it again

	if err := os.Rename(tempName, path); err != nil {
		return "", fmt.Errorf("failed to rename temp file to object path: %w", err)
	}
	success = true

	return hash, nil
}

// Get reads and decompresses the object with the given hash.
// Returns an error if the object does not exist.
func (s *Store) Get(hash string) ([]byte, error) {
	path := hashing.ObjectPath(s.twigDir, hash)
	compressed, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", hash)
		}
		return nil, fmt.Errorf("failed to read object file: %w", err)
	}

	decompressed, err := compress.Decompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress object %s: %w", hash, err)
	}

	return decompressed, nil
}
