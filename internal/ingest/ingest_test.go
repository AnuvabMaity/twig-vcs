package ingest

import (
	"bytes"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"twig/internal/objects"
	"twig/internal/store"
)

func TestBuildAsset(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-ingest-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// Generate 1.5MB of random data
	size := 1536 * 1024
	data := make([]byte, size)
	r := rand.New(rand.NewSource(42))
	if _, err := r.Read(data); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}

	// Build asset
	assetHash, err := BuildAsset(s, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("BuildAsset failed: %v", err)
	}

	// Retrieve encoded manifest
	manifestBytes, err := s.Get(assetHash)
	if err != nil {
		t.Fatalf("Get manifest failed: %v", err)
	}

	// Decode manifest
	var asset objects.Asset
	if err := objects.Decode(manifestBytes, &asset); err != nil {
		t.Fatalf("Decode manifest failed: %v", err)
	}

	if asset.Type != objects.TypeAsset {
		t.Errorf("Expected TypeAsset, got %s", asset.Type)
	}

	if asset.Size != uint64(size) {
		t.Errorf("Expected size %d, got %d", size, asset.Size)
	}

	// Retrieve and reconstruct each chunk
	var reconstructed []byte
	for i, ref := range asset.Chunks {
		chunkBytes, err := s.Get(ref.Hash)
		if err != nil {
			t.Fatalf("Get chunk %d failed: %v", i, err)
		}
		if len(chunkBytes) != int(ref.Size) {
			t.Errorf("Chunk %d size mismatch: expected %d, got %d", i, ref.Size, len(chunkBytes))
		}
		reconstructed = append(reconstructed, chunkBytes...)
	}

	if !bytes.Equal(data, reconstructed) {
		t.Errorf("Reconstructed asset does not match original data")
	}

	// Count number of files in store
	objectsDir := filepath.Join(tmpDir, "objects")
	initialFileCount := 0
	err = filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			initialFileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	// Deduplication test: storing the same asset again should write no new files
	assetHash2, err := BuildAsset(s, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Second BuildAsset failed: %v", err)
	}

	if assetHash != assetHash2 {
		t.Errorf("Expected identical asset hashes, got %s and %s", assetHash, assetHash2)
	}

	secondFileCount := 0
	err = filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			secondFileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir failed: %v", err)
	}

	if initialFileCount != secondFileCount {
		t.Errorf("Deduplication failed: expected %d files, found %d after second ingest", initialFileCount, secondFileCount)
	}
}

// TestIngestFileBoundaries verifies that files are correctly dispatched
// to either Blob or Asset depending on the 16KB threshold.
func TestIngestFileBoundaries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-ingest-boundary-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	tests := []struct {
		name         string
		size         int
		expectedType objects.ObjectType
	}{
		{
			name:         "Zero bytes (empty file)",
			size:         0,
			expectedType: objects.TypeBlob,
		},
		{
			name:         "Just under BlobThreshold (16KB - 1)",
			size:         objects.BlobThreshold - 1,
			expectedType: objects.TypeBlob,
		},
		{
			name:         "Exactly BlobThreshold (16KB)",
			size:         objects.BlobThreshold,
			expectedType: objects.TypeAsset,
		},
		{
			name:         "Just over BlobThreshold (16KB + 1)",
			size:         objects.BlobThreshold + 1,
			expectedType: objects.TypeAsset,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var filename string
			switch tc.size {
			case 0:
				filename = "zero.dat"
			case objects.BlobThreshold - 1:
				filename = "under.dat"
			case objects.BlobThreshold:
				filename = "exact.dat"
			case objects.BlobThreshold + 1:
				filename = "over.dat"
			}
			filePath := filepath.Join(tmpDir, filename)

			data := make([]byte, tc.size)
			for i := range data {
				data[i] = byte(i % 256)
			}

			if err := os.WriteFile(filePath, data, 0644); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			hash, objType, err := IngestFile(s, filePath)
			if err != nil {
				t.Fatalf("IngestFile failed: %v", err)
			}

			if objType != tc.expectedType {
				t.Errorf("Expected Type %s, got %s", tc.expectedType, objType)
			}

			// Verify we can retrieve it back and verify it matches for Blob
			if objType == objects.TypeBlob {
				retrievedBytes, err := s.Get(hash)
				if err != nil {
					t.Fatalf("Failed to retrieve Blob: %v", err)
				}
				var decodedBlob objects.Blob
				if err := objects.Decode(retrievedBytes, &decodedBlob); err != nil {
					t.Fatalf("Failed to decode Blob: %v", err)
				}
				if !bytes.Equal(data, decodedBlob.Data) {
					t.Errorf("Retrieved Blob data does not match original")
				}
			}
		})
	}
}

func TestReconstruct(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-reconstruct-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 1. Reconstruct Blob
	blobContent := []byte("simple small blob content")
	blobFile := filepath.Join(tmpDir, "blob.txt")
	if err := os.WriteFile(blobFile, blobContent, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	blobHash, blobType, err := IngestFile(s, blobFile)
	if err != nil {
		t.Fatalf("IngestFile failed: %v", err)
	}
	var blobBuf bytes.Buffer
	if err := Reconstruct(s, blobHash, blobType, &blobBuf); err != nil {
		t.Fatalf("Reconstruct failed: %v", err)
	}
	if !bytes.Equal(blobContent, blobBuf.Bytes()) {
		t.Errorf("Reconstructed blob content does not match original")
	}

	// 2. Reconstruct Asset (5+ chunks)
	// We want a large size to guarantee 5+ chunks.
	// Avg chunk size is 256KB, so 2MB (2048KB) should produce ~8 chunks.
	largeSize := 2 * 1024 * 1024
	largeContent := make([]byte, largeSize)
	r := rand.New(rand.NewSource(99))
	if _, err := r.Read(largeContent); err != nil {
		t.Fatalf("Failed to generate random content: %v", err)
	}
	largeFile := filepath.Join(tmpDir, "large.txt")
	if err := os.WriteFile(largeFile, largeContent, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	assetHash, assetType, err := IngestFile(s, largeFile)
	if err != nil {
		t.Fatalf("IngestFile failed: %v", err)
	}
	var assetBuf bytes.Buffer
	if err := Reconstruct(s, assetHash, assetType, &assetBuf); err != nil {
		t.Fatalf("Reconstruct failed: %v", err)
	}
	if !bytes.Equal(largeContent, assetBuf.Bytes()) {
		t.Errorf("Reconstructed asset content does not match original")
	}

	// 3. Unrecognized Type
	if err := Reconstruct(s, blobHash, objects.ObjectType("unrecognized"), &blobBuf); err == nil {
		t.Error("Expected error reconstructing unrecognized type, got nil")
	}

	// Decode asset manifest to get chunk hashes
	manifestBytes, err := s.Get(assetHash)
	if err != nil {
		t.Fatalf("Get manifest failed: %v", err)
	}
	var asset objects.Asset
	if err := objects.Decode(manifestBytes, &asset); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(asset.Chunks) < 5 {
		t.Errorf("Asset chunk count is %d, expected >= 5", len(asset.Chunks))
	}
	targetChunk := asset.Chunks[2]

	// 4. Corrupted/Truncated chunk size
	// Modify manifest in memory, write it to store, and reconstruct it.
	corruptedAsset := asset
	corruptedAsset.Chunks = make([]objects.ChunkRef, len(asset.Chunks))
	copy(corruptedAsset.Chunks, asset.Chunks)
	corruptedAsset.Chunks[2].Size += 10

	corruptedManifestBytes, err := objects.Encode(corruptedAsset)
	if err != nil {
		t.Fatalf("Failed to encode corrupted manifest: %v", err)
	}

	corruptedAssetHash, err := s.Put(corruptedManifestBytes)
	if err != nil {
		t.Fatalf("Failed to store corrupted manifest: %v", err)
	}

	var corruptBuf bytes.Buffer
	if err := Reconstruct(s, corruptedAssetHash, assetType, &corruptBuf); err == nil {
		t.Error("Expected error reconstructing asset with corrupted chunk size, got nil")
	} else if !strings.Contains(err.Error(), "chunk size mismatch") {
		t.Errorf("Expected chunk size mismatch error, got: %v", err)
	}

	// 5. Missing chunk
	chunkPath := filepath.Join(tmpDir, "objects", targetChunk.Hash[:2], targetChunk.Hash[2:])
	if err := os.Remove(chunkPath); err != nil {
		t.Fatalf("Failed to remove chunk file: %v", err)
	}

	var missingBuf bytes.Buffer
	if err := Reconstruct(s, assetHash, assetType, &missingBuf); err == nil {
		t.Error("Expected error reconstructing asset with missing chunk, got nil")
	}
}
