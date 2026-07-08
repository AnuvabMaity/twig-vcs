package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"twig/internal/objects"
	"twig/internal/refs"
	"twig/internal/repo"
	"twig/internal/store"
)

// runVizChunks implements 'bench viz chunks'
func runVizChunks(args []string) {
	fs := flag.NewFlagSet("viz chunks", flag.ExitOnError)
	oldRef := fs.String("old", "", "Old commit ref, branch, or hash")
	newRef := fs.String("new", "", "New commit ref, branch, or hash")
	filePath := fs.String("path", "", "Relative file path inside the repo")
	storeDir := fs.String("store", "", "Path to the .twig directory (optional)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *oldRef == "" || *newRef == "" || *filePath == "" {
		fmt.Fprintln(os.Stderr, "Usage: bench viz chunks --old <ref> --new <ref> --path <relpath> [--store DIR]")
		os.Exit(1)
	}

	var twigDir string
	var err error
	if *storeDir != "" {
		twigDir = *storeDir
	} else {
		twigDir, err = FindTwigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering repository: %v\n", err)
			os.Exit(1)
		}
	}

	st := store.Open(twigDir)

	// Resolve commit hashes
	oldCommit, err := ResolveRefOrHash(twigDir, *oldRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving --old reference %q: %v\n", *oldRef, err)
		os.Exit(1)
	}
	newCommit, err := ResolveRefOrHash(twigDir, *newRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving --new reference %q: %v\n", *newRef, err)
		os.Exit(1)
	}

	// Helper to find file in a commit
	getFileFromCommit := func(commitHash string) (string, objects.ObjectType, error) {
		commitBytes, err := st.Get(commitHash)
		if err != nil {
			return "", "", fmt.Errorf("failed to retrieve commit %s: %w", commitHash, err)
		}
		var c objects.Commit
		if err := objects.Decode(commitBytes, &c); err != nil {
			return "", "", fmt.Errorf("failed to decode commit %s: %w", commitHash, err)
		}
		files, err := repo.WalkTree(st, c.Root)
		if err != nil {
			return "", "", fmt.Errorf("failed to walk tree %s: %w", c.Root, err)
		}
		normalizedPath := filepath.ToSlash(*filePath)
		for _, f := range files {
			if f.Path == normalizedPath {
				return f.Hash, f.Type, nil
			}
		}
		return "", "", fmt.Errorf("file %q not found in commit %s", *filePath, commitHash)
	}

	oldFileHash, oldType, err := getFileFromCommit(oldCommit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in --old commit: %v\n", err)
		os.Exit(1)
	}
	newFileHash, newType, err := getFileFromCommit(newCommit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error in --new commit: %v\n", err)
		os.Exit(1)
	}

	// If both are blobs, it's just a single chunk comparison
	if oldType == objects.TypeBlob && newType == objects.TypeBlob {
		fmt.Printf("File: %s (Type: Blob, Size < 16KB)\n", *filePath)
		if oldFileHash == newFileHash {
			fmt.Println("Dedup ratio: 100% (file contents are identical)")
			fmt.Println("[●]")
		} else {
			fmt.Println("Dedup ratio: 0% (file contents differ completely)")
			fmt.Println("[○]")
		}
		return
	}

	// Retrieve assets
	var oldChunks []objects.ChunkRef
	if oldType == objects.TypeAsset {
		assetOldBytes, err := st.Get(oldFileHash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading old asset manifest: %v\n", err)
			os.Exit(1)
		}
		var assetOld objects.Asset
		if err := objects.Decode(assetOldBytes, &assetOld); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding old asset manifest: %v\n", err)
			os.Exit(1)
		}
		oldChunks = assetOld.Chunks
	} else {
		// Old was a blob, treat as a single chunk reference
		blobBytes, _ := st.Get(oldFileHash)
		oldChunks = []objects.ChunkRef{{Hash: oldFileHash, Size: uint32(len(blobBytes))}}
	}

	var newChunks []objects.ChunkRef
	var totalNewSize uint64
	if newType == objects.TypeAsset {
		assetNewBytes, err := st.Get(newFileHash)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading new asset manifest: %v\n", err)
			os.Exit(1)
		}
		var assetNew objects.Asset
		if err := objects.Decode(assetNewBytes, &assetNew); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding new asset manifest: %v\n", err)
			os.Exit(1)
		}
		newChunks = assetNew.Chunks
		totalNewSize = assetNew.Size
	} else {
		// New is a blob, treat as single chunk
		blobBytes, _ := st.Get(newFileHash)
		newChunks = []objects.ChunkRef{{Hash: newFileHash, Size: uint32(len(blobBytes))}}
		totalNewSize = uint64(len(blobBytes))
	}

	// Map old chunk hashes
	oldChunksMap := make(map[string]bool)
	for _, c := range oldChunks {
		oldChunksMap[c.Hash] = true
	}

	sharedChunksCount := 0
	var sharedBytes uint64
	newChunksCount := 0
	var newBytes uint64

	var asciiMap []string

	for _, c := range newChunks {
		if oldChunksMap[c.Hash] {
			sharedChunksCount++
			sharedBytes += uint64(c.Size)
			asciiMap = append(asciiMap, "●")
		} else {
			newChunksCount++
			newBytes += uint64(c.Size)
			asciiMap = append(asciiMap, "○")
		}
	}

	ratio := 0.0
	if totalNewSize > 0 {
		ratio = (float64(sharedBytes) / float64(totalNewSize)) * 100.0
	}

	fmt.Printf("File Chunk Visualizer: %s\n", *filePath)
	fmt.Printf("New Version Size:      %.2f MB (%d chunks)\n", float64(totalNewSize)/(1024*1024), len(newChunks))
	fmt.Printf("  └─ Shared/Reused:    %d chunks (%.2f MB)\n", sharedChunksCount, float64(sharedBytes)/(1024*1024))
	fmt.Printf("  └─ New/Unique:       %d chunks (%.2f MB)\n", newChunksCount, float64(newBytes)/(1024*1024))
	fmt.Printf("Deduplication Ratio:   %.2f%%\n", ratio)
	fmt.Println()
	fmt.Println("Chunk Mapping (● = reused, ○ = new):")
	fmt.Print("[")

	// Print chunk map wrapping lines cleanly
	for i, sym := range asciiMap {
		if i > 0 && i%60 == 0 {
			fmt.Println()
			fmt.Print(" ")
		}
		fmt.Print(sym)
	}
	fmt.Println("]")
}

// runVizStoreStats implements 'bench viz store-stats'
func runVizStoreStats(args []string) {
	fs := flag.NewFlagSet("viz store-stats", flag.ExitOnError)
	storeDir := fs.String("store", "", "Path to the .twig directory (optional)")
	raw := fs.Bool("raw", false, "Cheap directory scan only (total files/sizes, no classification)")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	var twigDir string
	var err error
	if *storeDir != "" {
		twigDir = *storeDir
	} else {
		twigDir, err = FindTwigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering repository: %v\n", err)
			os.Exit(1)
		}
	}

	objectsDir := filepath.Join(twigDir, objects.ObjectsDirName)

	// 1. Directory-only raw scan
	var totalFiles int64
	var totalBytes int64
	diskObjects := make(map[string]int64) // hash -> size

	err = filepath.WalkDir(objectsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		totalFiles++
		totalBytes += info.Size()

		rel, err := filepath.Rel(objectsDir, path)
		if err == nil {
			normalized := filepath.ToSlash(rel)
			parts := strings.Split(normalized, "/")
			if len(parts) == 2 && len(parts[0]) == 2 {
				hash := parts[0] + parts[1]
				if len(hash) == 64 {
					diskObjects[strings.ToLower(hash)] = info.Size()
				}
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning object database: %v\n", err)
		os.Exit(1)
	}

	if *raw {
		fmt.Println("--- Object Database Raw Stats ---")
		fmt.Printf("Total Loose Files: %d\n", totalFiles)
		fmt.Printf("Total Storage Size: %.2f MB (%d bytes)\n", float64(totalBytes)/(1024*1024), totalBytes)
		return
	}

	// 2. Graph-aware deep scan
	st := store.Open(twigDir)
	var roots []string

	// Resolve HEAD
	headCommit, err := refs.ResolveHEAD(twigDir)
	if err == nil && headCommit != "" {
		roots = append(roots, headCommit)
	}
	// List branches
	branches, err := refs.ListBranches(twigDir)
	if err == nil {
		for _, b := range branches {
			commitHash, err := refs.ReadBranch(twigDir, b)
			if err == nil && commitHash != "" {
				roots = append(roots, commitHash)
			}
		}
	}

	// Unique roots
	uniqueRootsMap := make(map[string]bool)
	var uniqueRoots []string
	for _, r := range roots {
		if !uniqueRootsMap[r] {
			uniqueRootsMap[r] = true
			uniqueRoots = append(uniqueRoots, r)
		}
	}

	visited := make(map[string]string) // hash -> type
	var chunkReferences int64

	var checkObject func(hash string, expectedType string)
	checkObject = func(hash string, expectedType string) {
		hash = strings.ToLower(hash)
		if _, ok := visited[hash]; ok {
			return
		}

		size, exists := diskObjects[hash]
		if !exists {
			visited[hash] = expectedType
			return
		}

		data, err := st.Get(hash)
		if err != nil {
			visited[hash] = expectedType
			return
		}

		// Auto-detect type
		var header struct {
			Type string `cbor:"type"`
		}
		detectedType := "chunk"
		if err := objects.Decode(data, &header); err == nil && header.Type != "" {
			detectedType = header.Type
		}

		visited[hash] = detectedType

		// Process sub-references
		switch detectedType {
		case "commit":
			var c objects.Commit
			_ = objects.Decode(data, &c)
			checkObject(c.Root, "tree")
			for _, p := range c.Parents {
				checkObject(p, "commit")
			}

		case "tree":
			var t objects.Tree
			_ = objects.Decode(data, &t)
			for _, entry := range t.Entries {
				checkObject(entry.Hash, string(entry.Type))
			}

		case "asset":
			var a objects.Asset
			_ = objects.Decode(data, &a)
			for _, chunk := range a.Chunks {
				chunkReferences++
				checkObject(chunk.Hash, "chunk")
			}
		}
		// Sizes of blobs and chunks are accumulated on disk
		_ = size
	}

	// Walk from roots
	for _, r := range uniqueRoots {
		checkObject(r, "commit")
	}

	// Separate reachable objects by type and compute sizes
	var commitsCount, treesCount, blobsCount, assetsCount, chunksCount int
	var commitsBytes, treesBytes, blobsBytes, assetsBytes, chunksBytes int64

	for hash, t := range visited {
		size := diskObjects[hash] // size on disk (compressed)
		switch t {
		case "commit":
			commitsCount++
			commitsBytes += size
		case "tree":
			treesCount++
			treesBytes += size
		case "blob":
			blobsCount++
			blobsBytes += size
		case "asset":
			assetsCount++
			assetsBytes += size
		case "chunk":
			chunksCount++
			chunksBytes += size
		}
	}

	// Orphans check
	var orphansCount int
	var orphansBytes int64
	for hash, size := range diskObjects {
		if _, reached := visited[hash]; !reached {
			orphansCount++
			orphansBytes += size
		}
	}

	fmt.Println("--- Reachable Storage Inventory ---")
	fmt.Printf("Commits:   %d  (size: %.2f KB)\n", commitsCount, float64(commitsBytes)/1024)
	fmt.Printf("Trees:     %d  (size: %.2f KB)\n", treesCount, float64(treesBytes)/1024)
	fmt.Printf("Blobs:     %d  (size: %.2f KB)\n", blobsCount, float64(blobsBytes)/1024)
	fmt.Printf("Assets:    %d  (size: %.2f KB)\n", assetsCount, float64(assetsBytes)/1024)
	fmt.Printf("Chunks:    %d  (size: %.2f MB, referenced %d times in manifests)\n", chunksCount, float64(chunksBytes)/(1024*1024), chunkReferences)
	fmt.Println()
	fmt.Printf("Orphaned Objects (unreferenced): %d (size: %.2f MB)\n", orphansCount, float64(orphansBytes)/(1024*1024))
	fmt.Printf("Total database disk size:       %.2f MB\n", float64(totalBytes)/(1024*1024))
}
