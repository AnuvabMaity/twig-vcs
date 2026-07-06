package hashing

import (
	"encoding/hex"
	"path/filepath"

	"github.com/zeebo/blake3"
)

// Hash returns the lowercase hex-encoded BLAKE3 digest of data.
func Hash(data []byte) string {
	sum := blake3.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// ObjectPath returns the on-disk path for an object with the given hash,
// rooted at twigDir (the .twig directory), using a 2-character prefix
// fan-out directory: <twigDir>/objects/<hash[:2]>/<hash[2:]>.
func ObjectPath(twigDir, hash string) string {
	if len(hash) < 2 {
		return filepath.Join(twigDir, "objects", hash)
	}
	prefix := hash[:2]
	rest := hash[2:]
	return filepath.Join(twigDir, "objects", prefix, rest)
}
