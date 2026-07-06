package objects

import (
	"bytes"
	"reflect"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	// 1. Blob
	blob := Blob{
		Type: TypeBlob,
		Data: []byte("test blob data"),
	}
	blobBytes, err := Encode(blob)
	if err != nil {
		t.Fatalf("failed to encode Blob: %v", err)
	}
	var decodedBlob Blob
	if err := Decode(blobBytes, &decodedBlob); err != nil {
		t.Fatalf("failed to decode Blob: %v", err)
	}
	if !reflect.DeepEqual(blob, decodedBlob) {
		t.Errorf("Blob round-trip failed. Expected %+v, got %+v", blob, decodedBlob)
	}

	// 2. Asset
	asset := Asset{
		Type: TypeAsset,
		Size: 12345,
		Chunks: []ChunkRef{
			{Hash: "chunkhash1", Size: 5000},
			{Hash: "chunkhash2", Size: 7345},
		},
	}
	assetBytes, err := Encode(asset)
	if err != nil {
		t.Fatalf("failed to encode Asset: %v", err)
	}
	var decodedAsset Asset
	if err := Decode(assetBytes, &decodedAsset); err != nil {
		t.Fatalf("failed to decode Asset: %v", err)
	}
	if !reflect.DeepEqual(asset, decodedAsset) {
		t.Errorf("Asset round-trip failed. Expected %+v, got %+v", asset, decodedAsset)
	}

	// 3. Tree
	tree := Tree{
		Type: TypeTree,
		Entries: []TreeEntry{
			{Name: "file1.txt", Hash: "hash1", Type: TypeBlob},
			{Name: "dir2", Hash: "hash2", Type: TypeTree},
		},
	}
	treeBytes, err := Encode(tree)
	if err != nil {
		t.Fatalf("failed to encode Tree: %v", err)
	}
	var decodedTree Tree
	if err := Decode(treeBytes, &decodedTree); err != nil {
		t.Fatalf("failed to decode Tree: %v", err)
	}
	if !reflect.DeepEqual(tree, decodedTree) {
		t.Errorf("Tree round-trip failed. Expected %+v, got %+v", tree, decodedTree)
	}

	// 4. Commit
	commit := Commit{
		Type:    TypeCommit,
		Root:    "roothash",
		Parents: []string{"parent1", "parent2"},
		Author: Author{
			ID:   "alice",
			Time: 1600000000,
		},
		Message: "Initial commit",
	}
	commitBytes, err := Encode(commit)
	if err != nil {
		t.Fatalf("failed to encode Commit: %v", err)
	}
	var decodedCommit Commit
	if err := Decode(commitBytes, &decodedCommit); err != nil {
		t.Fatalf("failed to decode Commit: %v", err)
	}
	if !reflect.DeepEqual(commit, decodedCommit) {
		t.Errorf("Commit round-trip failed. Expected %+v, got %+v", commit, decodedCommit)
	}
}

func TestCodecDeterminism(t *testing.T) {
	// Construct a Tree object in two different ways.
	// In Go, struct field order is fixed by the type definition, but map key order in nested structures (like a map field, if any) or similar ordering could vary.
	// Also, we want to prove that the exact same logical data structure marshals deterministically.
	// Since CBOR canonical mode sorts map keys deterministically, if we encode a struct with maps or fields, it is byte-identical.
	// Let's create an object that has map elements or similar to test map key sorting deterministically.
	// Even though Tree doesn't use a map, let's test a dummy map to prove canonical encoding works.
	type MapStruct struct {
		Fields map[string]int `cbor:"fields"`
	}

	// Two identical maps but populated in different orders or memory layouts
	obj1 := MapStruct{
		Fields: map[string]int{
			"zebra": 1,
			"apple": 2,
			"cat":   3,
		},
	}
	obj2 := MapStruct{
		Fields: map[string]int{
			"apple": 2,
			"cat":   3,
			"zebra": 1,
		},
	}

	bytes1, err := Encode(obj1)
	if err != nil {
		t.Fatalf("failed to encode obj1: %v", err)
	}

	bytes2, err := Encode(obj2)
	if err != nil {
		t.Fatalf("failed to encode obj2: %v", err)
	}

	if !bytes.Equal(bytes1, bytes2) {
		t.Errorf("Map canonical sorting failed. Bytes 1: %x, Bytes 2: %x", bytes1, bytes2)
	}
}

func TestCodecLoopDeterminism(t *testing.T) {
	commit := Commit{
		Type:    TypeCommit,
		Root:    "roothash",
		Parents: []string{"parent1", "parent2"},
		Author: Author{
			ID:   "alice",
			Time: 1600000000,
		},
		Message: "Initial commit",
	}

	firstBytes, err := Encode(commit)
	if err != nil {
		t.Fatalf("failed to encode Commit on first try: %v", err)
	}

	for i := 0; i < 100; i++ {
		currentBytes, err := Encode(commit)
		if err != nil {
			t.Fatalf("failed to encode Commit on iteration %d: %v", i, err)
		}
		if !bytes.Equal(firstBytes, currentBytes) {
			t.Fatalf("Non-deterministic encoding on iteration %d. Expected: %x, got: %x", i, firstBytes, currentBytes)
		}
	}
}
