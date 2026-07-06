package compress

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestZstdRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "Empty input",
			data: []byte{},
		},
		{
			name: "Small input",
			data: []byte("hello world this is a small zstd test byte slice"),
		},
		{
			name: "Over 1MB input",
			data: generateLargeData(1*1024*1024 + 100), // 1MB + 100 bytes
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			compressed, err := Compress(tc.data)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			decompressed, err := Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}

			if !bytes.Equal(tc.data, decompressed) {
				t.Errorf("Round-trip failed. Data doesn't match original.")
			}
		})
	}
}

func TestDecompressInvalidData(t *testing.T) {
	invalidData := []byte("this is definitely not compressed zstd data")
	_, err := Decompress(invalidData)
	if err == nil {
		t.Errorf("Expected error when decompressing invalid data, got nil")
	}
}

// generateLargeData generates size bytes of pseudo-random compressible data.
func generateLargeData(size int) []byte {
	// Repeat a sequence of characters to ensure it's easily compressible
	src := []byte("abcdefghijklmnopqrstuvwxyz0123456789")
	data := make([]byte, size)
	r := rand.New(rand.NewSource(42))
	for i := 0; i < size; {
		chunkSize := r.Intn(100) + 10
		if i+chunkSize > size {
			chunkSize = size - i
		}
		// Repeat some pattern
		for j := 0; j < chunkSize; j++ {
			data[i+j] = src[(i+j)%len(src)]
		}
		i += chunkSize
	}
	return data
}
