package ingest

import (
	"fmt"
	"io"
	"os"

	"twig/internal/chunker"
	"twig/internal/objects"
	"twig/internal/store"
)

// BuildAsset chunks r, stores each chunk in s (deduping automatically via
// store.Put), and stores an Asset manifest referencing them in order.
// Returns the hash of the stored Asset object.
func BuildAsset(s *store.Store, r io.Reader) (string, error) {
	chunks, err := chunker.Split(r)
	if err != nil {
		return "", err
	}

	var chunkRefs []objects.ChunkRef
	var totalSize uint64
	for _, chunk := range chunks {
		hash, err := s.Put(chunk)
		if err != nil {
			return "", err
		}
		chunkRefs = append(chunkRefs, objects.ChunkRef{
			Hash: hash,
			Size: uint32(len(chunk)),
		})
		totalSize += uint64(len(chunk))
	}

	asset := objects.Asset{
		Type:   objects.TypeAsset,
		Size:   totalSize,
		Chunks: chunkRefs,
	}

	encoded, err := objects.Encode(asset)
	if err != nil {
		return "", err
	}

	assetHash, err := s.Put(encoded)
	if err != nil {
		return "", err
	}

	return assetHash, nil
}

// IngestFile reads the file at path and stores it as a Blob (if smaller
// than objects.BlobThreshold) or an Asset (otherwise). Returns the
// resulting object's hash and type.
func IngestFile(s *store.Store, path string) (string, objects.ObjectType, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return "", "", fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	size := fi.Size()
	// Boundary is >= (16KB or larger is stored as an Asset, under 16KB is a Blob).
	if size >= int64(objects.BlobThreshold) {
		hash, err := BuildAsset(s, file)
		if err != nil {
			return "", "", err
		}
		return hash, objects.TypeAsset, nil
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file %s: %w", path, err)
	}

	blob := objects.Blob{
		Type: objects.TypeBlob,
		Data: data,
	}

	encoded, err := objects.Encode(blob)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode blob: %w", err)
	}

	hash, err := s.Put(encoded)
	if err != nil {
		return "", "", err
	}

	return hash, objects.TypeBlob, nil
}

// Reconstruct writes the original content for the object with the given
// hash and type to w. For a Blob, this is a direct write. For an Asset,
// this reads each referenced chunk in manifest order and writes it in
// sequence.
func Reconstruct(s *store.Store, hash string, objType objects.ObjectType, w io.Writer) error {
	switch objType {
	case objects.TypeBlob:
		bytesFetched, err := s.Get(hash)
		if err != nil {
			return fmt.Errorf("failed to retrieve blob %s: %w", hash, err)
		}
		var blob objects.Blob
		if err := objects.Decode(bytesFetched, &blob); err != nil {
			return fmt.Errorf("failed to decode blob %s: %w", hash, err)
		}
		if _, err := w.Write(blob.Data); err != nil {
			return fmt.Errorf("failed to write blob content to output stream: %w", err)
		}
		return nil

	case objects.TypeAsset:
		manifestBytes, err := s.Get(hash)
		if err != nil {
			return fmt.Errorf("failed to retrieve asset manifest %s: %w", hash, err)
		}
		var asset objects.Asset
		if err := objects.Decode(manifestBytes, &asset); err != nil {
			return fmt.Errorf("failed to decode asset manifest %s: %w", hash, err)
		}
		for i, ref := range asset.Chunks {
			chunkBytes, err := s.Get(ref.Hash)
			if err != nil {
				return fmt.Errorf("failed to retrieve chunk %d (%s) of asset %s: %w", i, ref.Hash, hash, err)
			}
			if len(chunkBytes) != int(ref.Size) {
				return fmt.Errorf("chunk size mismatch for chunk %d (%s) of asset %s: expected %d, got %d", i, ref.Hash, hash, ref.Size, len(chunkBytes))
			}
			if _, err := w.Write(chunkBytes); err != nil {
				return fmt.Errorf("failed to write chunk %d data to output stream: %w", i, err)
			}
		}
		return nil

	default:
		return fmt.Errorf("unrecognized object type: %s", objType)
	}
}
