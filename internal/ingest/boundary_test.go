package ingest

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/objects"
	"twig/internal/store"
)

func TestBoundaryBlobThreshold(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-boundary-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 1. File exactly at BlobThreshold (16KB)
	// Must be stored as objects.TypeAsset
	sizeExact := objects.BlobThreshold
	exactFile := filepath.Join(tmpDir, "exact.dat")
	dataExact := make([]byte, sizeExact)
	for i := range dataExact {
		dataExact[i] = byte(i % 256)
	}
	if err := os.WriteFile(exactFile, dataExact, 0644); err != nil {
		t.Fatalf("Failed to write exact file: %v", err)
	}

	hashExact, typeExact, err := IngestFile(s, exactFile)
	if err != nil {
		t.Fatalf("IngestFile failed: %v", err)
	}
	if typeExact != objects.TypeAsset {
		t.Errorf("Expected exactly BlobThreshold to be TypeAsset, got %s", typeExact)
	}

	var reconstructExact bytes.Buffer
	if err := Reconstruct(s, hashExact, typeExact, &reconstructExact); err != nil {
		t.Fatalf("Reconstruct failed: %v", err)
	}
	if !bytes.Equal(dataExact, reconstructExact.Bytes()) {
		t.Errorf("Reconstructed content mismatch for BlobThreshold size")
	}
}

func TestBoundaryBetweenThresholdAndMinChunk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-boundary-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 2. File smaller than ChunkMinSize (64KB) but larger than BlobThreshold (16KB)
	// Let's use 32KB. Should become a single-chunk Asset.
	sizeMid := 32 * 1024
	midFile := filepath.Join(tmpDir, "mid.dat")
	dataMid := make([]byte, sizeMid)
	for i := range dataMid {
		dataMid[i] = byte(i % 256)
	}
	if err := os.WriteFile(midFile, dataMid, 0644); err != nil {
		t.Fatalf("Failed to write mid file: %v", err)
	}

	hashMid, typeMid, err := IngestFile(s, midFile)
	if err != nil {
		t.Fatalf("IngestFile failed: %v", err)
	}
	if typeMid != objects.TypeAsset {
		t.Fatalf("Expected 32KB file to be TypeAsset, got %s", typeMid)
	}

	// Retrieve manifest and verify it's a single-chunk Asset
	manifestBytes, err := s.Get(hashMid)
	if err != nil {
		t.Fatalf("Get manifest failed: %v", err)
	}
	var asset objects.Asset
	if err := objects.Decode(manifestBytes, &asset); err != nil {
		t.Fatalf("Decode manifest failed: %v", err)
	}
	if len(asset.Chunks) != 1 {
		t.Errorf("Expected exactly 1 chunk for 32KB file, got %d", len(asset.Chunks))
	}

	var reconstructMid bytes.Buffer
	if err := Reconstruct(s, hashMid, typeMid, &reconstructMid); err != nil {
		t.Fatalf("Reconstruct failed: %v", err)
	}
	if !bytes.Equal(dataMid, reconstructMid.Bytes()) {
		t.Errorf("Reconstructed content mismatch for 32KB file")
	}
}

func TestBoundaryEmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-boundary-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 3. Empty file (0 bytes) end-to-end through IngestFile and Reconstruct
	emptyFile := filepath.Join(tmpDir, "empty.dat")
	if err := os.WriteFile(emptyFile, nil, 0644); err != nil {
		t.Fatalf("Failed to write empty file: %v", err)
	}

	hashEmpty, typeEmpty, err := IngestFile(s, emptyFile)
	if err != nil {
		t.Fatalf("IngestFile failed: %v", err)
	}
	if typeEmpty != objects.TypeBlob {
		t.Errorf("Expected empty file to be TypeBlob, got %s", typeEmpty)
	}

	var reconstructEmpty bytes.Buffer
	if err := Reconstruct(s, hashEmpty, typeEmpty, &reconstructEmpty); err != nil {
		t.Fatalf("Reconstruct failed: %v", err)
	}
	if reconstructEmpty.Len() != 0 {
		t.Errorf("Expected reconstructed empty file to have 0 length, got %d", reconstructEmpty.Len())
	}
}

func TestBoundaryNonMultipleSize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-boundary-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 4. File whose size is not an exact multiple of any chunk boundary (e.g. 500KB + 13 bytes).
	// Verify that the last chunk can be smaller than ChunkMinSize (64KB), and it reconstructs correctly.
	sizeLarge := 500*1024 + 13
	largeFile := filepath.Join(tmpDir, "large.dat")
	dataLarge := make([]byte, sizeLarge)
	r := rand.New(rand.NewSource(777))
	if _, err := r.Read(dataLarge); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}
	if err := os.WriteFile(largeFile, dataLarge, 0644); err != nil {
		t.Fatalf("Failed to write large file: %v", err)
	}

	hashLarge, typeLarge, err := IngestFile(s, largeFile)
	if err != nil {
		t.Fatalf("IngestFile failed: %v", err)
	}
	if typeLarge != objects.TypeAsset {
		t.Fatalf("Expected 500KB+13B file to be TypeAsset, got %s", typeLarge)
	}

	// Get manifest and check the last chunk size
	manifestBytes, err := s.Get(hashLarge)
	if err != nil {
		t.Fatalf("Get manifest failed: %v", err)
	}
	var asset objects.Asset
	if err := objects.Decode(manifestBytes, &asset); err != nil {
		t.Fatalf("Decode manifest failed: %v", err)
	}

	if len(asset.Chunks) <= 1 {
		t.Fatalf("Expected multiple chunks, got %d", len(asset.Chunks))
	}

	lastChunk := asset.Chunks[len(asset.Chunks)-1]
	t.Logf("Last chunk size is %d bytes", lastChunk.Size)

	var reconstructLarge bytes.Buffer
	if err := Reconstruct(s, hashLarge, typeLarge, &reconstructLarge); err != nil {
		t.Fatalf("Reconstruct failed: %v", err)
	}
	if !bytes.Equal(dataLarge, reconstructLarge.Bytes()) {
		t.Errorf("Reconstructed content mismatch for non-multiple size file")
	}
}
