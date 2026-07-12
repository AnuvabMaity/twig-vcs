package objects

// FormatVersion is the version of the on-disk storage format.
const FormatVersion = 1

// Chunker constants configuration.
const (
	ChunkMinSize = 64 * 1024   // 64 KB
	ChunkAvgSize = 256 * 1024  // 256 KB
	ChunkMaxSize = 1024 * 1024 // 1 MB
)

// BlobThreshold defines the size boundary for storing files as Assets vs Blobs.
const BlobThreshold = ChunkMinSize // 64 KB

const (
	DefaultTwigDir    = ".twig"   // repository metadata directory name (e.g. ".twig").
	IndexFileName     = "index"   // name of the staging index file.
	ConfigFileName    = "config"  // name of the repository config file.
	HeadFileName      = "HEAD"    // symbolic reference tracking file.
	VersionFileName   = "VERSION" // file containing the layout schema version.
	ObjectsDirName    = "objects" // folder containing loose object databases.
	RefsDirName       = "refs"    // directory containing references.
	HeadsDirName      = "heads"   // subdirectory containing branch head refs.
	DefaultBranchName = "main"    // default branch initialized in a new repo.

	// Permissions
	DirPermMode  = 0755
	FilePermMode = 0644
)
