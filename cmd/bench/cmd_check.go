package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"twig/internal/hashing"
	"twig/internal/objects"
	"twig/internal/refs"
	"twig/internal/store"
)

// runCheckIntegrity implements 'bench check integrity'
func runCheckIntegrity(args []string) {
	fs := flag.NewFlagSet("check integrity", flag.ExitOnError)
	storeDir := fs.String("store", "", "Path to the .twig directory (optional, auto-discovered if empty)")
	verbose := fs.Bool("verbose", false, "Print detailed verification progress")

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

	st := store.Open(twigDir)

	// 1. Gather all physical object files from the disk
	diskObjects := make(map[string]bool)
	objectsDir := filepath.Join(twigDir, objects.ObjectsDirName)
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
		// Rel path should look like <prefixDir>/<rest>
		rel, err := filepath.Rel(objectsDir, path)
		if err != nil {
			return err
		}
		normalized := filepath.ToSlash(rel)
		parts := strings.Split(normalized, "/")
		if len(parts) == 2 && len(parts[0]) == 2 {
			hash := parts[0] + parts[1]
			if len(hash) == 64 {
				diskObjects[strings.ToLower(hash)] = true
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error scanning objects on disk: %v\n", err)
		os.Exit(1)
	}

	// 2. Discover verification entry points (branches and HEAD)
	var roots []string

	// Resolve HEAD
	headCommit, err := refs.ResolveHEAD(twigDir)
	if err == nil && headCommit != "" {
		roots = append(roots, headCommit)
	} else if err != nil && !strings.Contains(err.Error(), "branch has no commits yet") {
		fmt.Fprintf(os.Stderr, "Warning: failed to resolve HEAD ref: %v\n", err)
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
	} else {
		fmt.Fprintf(os.Stderr, "Warning: failed to list branches: %v\n", err)
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

	if len(uniqueRoots) == 0 {
		fmt.Println("No commits found in references or HEAD. Verification completed (empty repo).")
		return
	}

	// 3. Perform recursive graph walk and check integrity
	visited := make(map[string]string) // hash -> type
	errorsList := []string{}

	var checkObject func(hash string, expectedType string)
	checkObject = func(hash string, expectedType string) {
		hash = strings.ToLower(hash)
		if t, ok := visited[hash]; ok {
			if expectedType != "" && t != expectedType && expectedType != "chunk" {
				errorsList = append(errorsList, fmt.Sprintf("Object type mismatch for %s: traversed as %s, previously seen as %s", hash, expectedType, t))
			}
			return
		}

		// Verify file exists on disk
		if !diskObjects[hash] {
			errorsList = append(errorsList, fmt.Sprintf("Missing object: %s (expected type: %s)", hash, expectedType))
			visited[hash] = expectedType
			return
		}

		// Read decompressed bytes
		data, err := st.Get(hash)
		if err != nil {
			errorsList = append(errorsList, fmt.Sprintf("Failed to read/decompress object %s: %v", hash, err))
			visited[hash] = expectedType
			return
		}

		// Integrity check: re-hash the decompressed content and verify checksum
		// Wait! st.Get(hash) returns decompressed content.
		// Wait, st.Put(content) computes hashing.Hash(content), compresses it, and writes it.
		// So hashing.Hash(decompressed) must EXACTLY match the filename hash!
		computedHash := hashing.Hash(data)
		if computedHash != hash {
			errorsList = append(errorsList, fmt.Sprintf("Corruption detected: object %s re-hashes to %s (integrity violation)", hash, computedHash))
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

		if *verbose {
			fmt.Printf("Verifying %s: %s (size: %d bytes)\n", detectedType, hash, len(data))
		}

		// Process sub-references
		switch detectedType {
		case "commit":
			var c objects.Commit
			if err := objects.Decode(data, &c); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("Failed to decode commit %s: %v", hash, err))
				return
			}
			// Root tree must exist and be of type "tree"
			checkObject(c.Root, "tree")
			// Parents must exist and be of type "commit"
			for _, p := range c.Parents {
				checkObject(p, "commit")
			}

		case "tree":
			var t objects.Tree
			if err := objects.Decode(data, &t); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("Failed to decode tree %s: %v", hash, err))
				return
			}

			// Verify lexicographical order of entries by Name
			if !sort.SliceIsSorted(t.Entries, func(i, j int) bool {
				return t.Entries[i].Name < t.Entries[j].Name
			}) {
				errorsList = append(errorsList, fmt.Sprintf("Non-canonical Tree layout: entries in tree %s are not sorted alphabetically", hash))
			}

			// Validate entries
			for _, entry := range t.Entries {
				checkObject(entry.Hash, string(entry.Type))
			}

		case "asset":
			var a objects.Asset
			if err := objects.Decode(data, &a); err != nil {
				errorsList = append(errorsList, fmt.Sprintf("Failed to decode asset manifest %s: %v", hash, err))
				return
			}
			// Chunks must exist and be raw chunks
			for _, chunk := range a.Chunks {
				checkObject(chunk.Hash, "chunk")
			}

		case "blob":
			// Leaf node, nothing to verify inside

		case "chunk":
			// Leaf node, nothing to verify inside
		}
	}

	// Run walk starting from all discovered roots
	for _, root := range uniqueRoots {
		checkObject(root, "commit")
	}

	// 4. Find orphaned objects (loose on disk but not reachable from any ref)
	var orphans []string
	for hash := range diskObjects {
		if _, reached := visited[hash]; !reached {
			orphans = append(orphans, hash)
		}
	}

	// Sort orphans for deterministic output
	sort.Strings(orphans)

	// 5. Present report
	fmt.Println("--- Integrity Report ---")
	fmt.Printf("Reachable Objects Verified: %d\n", len(visited))
	var countCommits, countTrees, countBlobs, countAssets, countChunks int
	for _, t := range visited {
		switch t {
		case "commit":
			countCommits++
		case "tree":
			countTrees++
		case "blob":
			countBlobs++
		case "asset":
			countAssets++
		case "chunk":
			countChunks++
		}
	}
	fmt.Printf("  └─ Commits: %d\n", countCommits)
	fmt.Printf("  └─ Trees:   %d\n", countTrees)
	fmt.Printf("  └─ Blobs:   %d\n", countBlobs)
	fmt.Printf("  └─ Assets:  %d\n", countAssets)
	fmt.Printf("  └─ Chunks:  %d\n", countChunks)

	if len(orphans) > 0 {
		fmt.Printf("Orphaned Objects Found (unreachable from references): %d\n", len(orphans))
		if *verbose {
			for _, o := range orphans {
				fmt.Printf("  └─ %s\n", o)
			}
		} else {
			fmt.Println("  (run with --verbose to list all orphaned hashes)")
		}
	} else {
		fmt.Println("Orphaned Objects Found: 0")
	}

	if len(errorsList) > 0 {
		fmt.Printf("\n❌ INTEGRITY FAILURE: %d anomalies detected!\n", len(errorsList))
		for _, errStr := range errorsList {
			fmt.Printf("  [!] %s\n", errStr)
		}
		os.Exit(1)
	} else {
		fmt.Println("\n✅ INTEGRITY OK: Repository is structurally sound.")
	}
}
