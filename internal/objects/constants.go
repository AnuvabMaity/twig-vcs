package objects

// FormatVersion is the version of the on-disk storage format.
const FormatVersion = 1

// Chunker constants configuration.
const (
	ChunkMinSize = 64 * 1024   // 64 KB
	ChunkAvgSize = 256 * 1024  // 256 KB
	ChunkMaxSize = 1024 * 1024 // 1 MB
)

// BlobThreshold defines the size boundary (16 KB) for storing files as Assets vs Blobs.
const BlobThreshold = 16 * 1024 // 16 KB
