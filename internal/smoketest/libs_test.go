package smoketest

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/jotfs/fastcdc-go"
	"github.com/klauspost/compress/zstd"
	"github.com/zeebo/blake3"
)

func TestBlake3(t *testing.T) {
	input := []byte("hello")
	// The BLAKE3 hash of "hello"
	hasher := blake3.New()
	hasher.Write(input)
	sum := hasher.Sum(nil)
	hexHash := hex.EncodeToString(sum)

	t.Logf("BLAKE3 hash of 'hello': %s", hexHash)

	// Verify length (32 bytes = 64 hex characters)
	if len(hexHash) != 64 {
		t.Errorf("Expected 64 character hex string, got %d", len(hexHash))
	}

	// BLAKE3 hex hash for "hello" is:
	// ea8f163db38682925e4491c5e58d4bb3506ef8c14eb78a86e908c5624a67200f
	expected := "ea8f163db38682925e4491c5e58d4bb3506ef8c14eb78a86e908c5624a67200f"
	if hexHash != expected {
		t.Errorf("Expected BLAKE3 hash %s, got %s", expected, hexHash)
	}
}

func TestFastCDC(t *testing.T) {
	// Let's chunk a small in-memory byte slice and print the number of chunks produced.
	opts := fastcdc.Options{
		MinSize:     1024,
		AverageSize: 2048,
		MaxSize:     4096,
	}

	// Generate 10KB of data
	data := make([]byte, 10*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	chunker, err := fastcdc.NewChunker(bytes.NewReader(data), opts)
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}

	chunkCount := 0
	var reconstructed []byte
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Error reading chunk: %v", err)
		}
		chunkCount++
		reconstructed = append(reconstructed, chunk.Data...)
	}

	t.Logf("FastCDC chunk count: %d", chunkCount)
	if chunkCount == 0 {
		t.Error("Expected at least one chunk")
	}
	if !bytes.Equal(data, reconstructed) {
		t.Error("Reconstructed data does not match original")
	}
}

func TestCBOR(t *testing.T) {
	type DummyStruct struct {
		Name  string            `cbor:"name"`
		Value int               `cbor:"value"`
		Meta  map[string]string `cbor:"meta"`
	}

	val1 := DummyStruct{
		Name:  "test",
		Value: 42,
		Meta: map[string]string{
			"key1": "val1",
			"key2": "val2",
		},
	}

	val2 := DummyStruct{
		Name:  "test",
		Value: 42,
		Meta: map[string]string{
			"key2": "val2",
			"key1": "val1",
		},
	}

	// We create a canonical CBOR encoder mode
	em, err := cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		t.Fatalf("Failed to create canonical enc mode: %v", err)
	}

	enc1, err := em.Marshal(val1)
	if err != nil {
		t.Fatalf("Failed to marshal val1: %v", err)
	}

	enc2, err := em.Marshal(val2)
	if err != nil {
		t.Fatalf("Failed to marshal val2: %v", err)
	}

	// Since maps might have keys in different order in memory,
	// canonical encoding must produce exactly the same bytes.
	if !bytes.Equal(enc1, enc2) {
		t.Errorf("Canonical encoding is not identical. Enc1: %x, Enc2: %x", enc1, enc2)
	}

	var decoded DummyStruct
	err = cbor.Unmarshal(enc1, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal CBOR: %v", err)
	}

	if decoded.Name != val1.Name || decoded.Value != val1.Value || decoded.Meta["key1"] != val1.Meta["key1"] || decoded.Meta["key2"] != val1.Meta["key2"] {
		t.Errorf("Round-trip decoded struct does not match original: %+v", decoded)
	}
}

func TestZstd(t *testing.T) {
	input := []byte("This is a test byte slice that we want to compress using zstd. It needs to be repetitive so it actually compresses well. repeating repeating repeating repeating repeating repeating repeating repeating repeating repeating repeating")

	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatalf("Failed to create zstd writer: %v", err)
	}
	defer encoder.Close()

	compressed := encoder.EncodeAll(input, nil)
	t.Logf("Original size: %d, compressed size: %d", len(input), len(compressed))

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		t.Fatalf("Failed to create zstd reader: %v", err)
	}
	defer decoder.Close()

	decompressed, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		t.Fatalf("Failed to decode zstd data: %v", err)
	}

	if !bytes.Equal(input, decompressed) {
		t.Error("Decompressed data does not match input")
	}
}
