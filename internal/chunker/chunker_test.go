package chunker

import (
	"bytes"
	"math/rand"
	"testing"

	"twig/internal/objects"
)

func TestSplitLargeFile(t *testing.T) {
	// Generate 3MB of pseudo-random data to ensure multiple chunks are created
	size := 3 * 1024 * 1024
	data := make([]byte, size)
	r := rand.New(rand.NewSource(42))
	if _, err := r.Read(data); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}

	chunks, err := Split(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) <= 1 {
		t.Errorf("Expected multiple chunks for 3MB data, got %d", len(chunks))
	}

	// Verify size constraints and reconstruction
	var reconstructed []byte
	for i, chunk := range chunks {
		chunkLen := len(chunk)
		reconstructed = append(reconstructed, chunk...)

		// Verify constraints: min/max size apply to all except potentially the last chunk
		if i < len(chunks)-1 {
			if chunkLen < objects.ChunkMinSize || chunkLen > objects.ChunkMaxSize {
				t.Errorf("Chunk %d size %d out of bounds [%d, %d]", i, chunkLen, objects.ChunkMinSize, objects.ChunkMaxSize)
			}
		} else {
			// Last chunk must not exceed MaxSize
			if chunkLen > objects.ChunkMaxSize {
				t.Errorf("Last chunk size %d exceeds MaxSize %d", chunkLen, objects.ChunkMaxSize)
			}
		}
	}

	if !bytes.Equal(data, reconstructed) {
		t.Errorf("Reconstructed data does not match original data")
	}
}

func TestSplitSmallFile(t *testing.T) {
	// Generate 32KB of data (less than ChunkMinSize = 64KB)
	size := 32 * 1024
	data := make([]byte, size)
	r := rand.New(rand.NewSource(42))
	if _, err := r.Read(data); err != nil {
		t.Fatalf("Failed to generate random data: %v", err)
	}

	chunks, err := Split(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("Expected exactly 1 chunk for 32KB data, got %d", len(chunks))
	}

	if !bytes.Equal(data, chunks[0]) {
		t.Errorf("Chunk data does not match original small file content")
	}
}

func TestSplitEmpty(t *testing.T) {
	// Splitting empty input should yield zero chunks and not panic.
	chunks, err := Split(bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("Split failed on empty input: %v", err)
	}

	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty input, got %d", len(chunks))
	}
}
