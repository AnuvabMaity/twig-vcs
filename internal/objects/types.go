package objects

import "sort"

// ObjectType represents the type of a repository object.
type ObjectType string

const (
	// TypeBlob represents a basic file data object (under 16KB).
	TypeBlob ObjectType = "blob"
	// TypeAsset represents a chunked file data object (16KB or larger).
	TypeAsset ObjectType = "asset"
	// TypeTree represents a directory listing object.
	TypeTree ObjectType = "tree"
	// TypeCommit represents a commit node containing author info, root tree hash, and parent hash(es).
	TypeCommit ObjectType = "commit"
)

// ChunkRef represents a reference to a content-defined chunk of an asset.
type ChunkRef struct {
	Hash string `cbor:"hash"`
	Size uint32 `cbor:"size"`
}

// Blob represents a basic file object.
type Blob struct {
	Type ObjectType `cbor:"type"`
	Data []byte     `cbor:"data"`
}

// Asset represents a chunked file object.
type Asset struct {
	Type   ObjectType `cbor:"type"`
	Size   uint64     `cbor:"size"`
	Chunks []ChunkRef `cbor:"chunks"`
}

// TreeEntry represents an entry inside a Tree directory listing.
type TreeEntry struct {
	Name string     `cbor:"name"`
	Hash string     `cbor:"hash"`
	Type ObjectType `cbor:"type"` // TypeBlob, TypeAsset, or TypeTree
}

// Tree represents a directory listing, mapping names to object hashes.
type Tree struct {
	Type    ObjectType  `cbor:"type"`
	Entries []TreeEntry `cbor:"entries"` // must be sorted by Name before encoding
}

// Author represents the creator metadata of a commit.
type Author struct {
	ID   string `cbor:"id"`
	Time int64  `cbor:"time"` // Unix seconds
}

// Commit represents a point-in-time repository snapshot.
type Commit struct {
	Type    ObjectType `cbor:"type"`
	Root    string     `cbor:"root"`
	Parents []string   `cbor:"parents"`
	Author  Author     `cbor:"author"`
	Message string     `cbor:"msg"`
}

// SortTreeEntries sorts a slice of TreeEntry in place by their Name field.
func SortTreeEntries(entries []TreeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
}
