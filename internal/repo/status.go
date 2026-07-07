package repo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"twig/internal/index"
	"twig/internal/ingest"
	"twig/internal/objects"
	"twig/internal/refs"
)

// FileStatus represents the state of a file in the repository.
type FileStatus string

const (
	StatusUntracked      FileStatus = "untracked"       // on disk, not in the index at all
	StatusModified       FileStatus = "modified"        // in the index, but working-dir content has changed since it was staged
	StatusDeleted        FileStatus = "deleted"          // in the index, but missing from disk
	StatusStagedNew      FileStatus = "staged-new"       // in the index, not present in HEAD's tree
	StatusStagedModified FileStatus = "staged-modified"  // in the index, present in HEAD's tree, but with a different hash
	StatusUnmodified     FileStatus = "unmodified"       // identical across working dir, index, and HEAD
	StatusConflict       FileStatus = "conflict"         // has conflict markers in the index
)

// StatusEntry records the status of a specific file path.
type StatusEntry struct {
	Path   string
	Status FileStatus
}

// Status compares the working directory, the staging index, and HEAD's
// tree, and returns one StatusEntry per path that needs reporting. A
// single path CAN appear twice (e.g. once as StatusStagedModified,
// relative to HEAD, and again as StatusModified, if it was staged and
// then edited again afterward) — this mirrors how Git reports the same
// file under both "Changes to be committed" and "Changes not staged"
// when that happens. Document this explicitly wherever Status's result
// is consumed, so nobody assumes a 1:1 path-to-entry mapping.
func (r *Repo) Status() ([]StatusEntry, error) {
	// 1. Load Staging Index
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	idx, err := index.Load(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	// 2. Resolve HEAD's Tree Files
	headFiles := make(map[string]string) // path -> hash
	headCommitHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		if !errors.Is(err, refs.ErrUnbornBranch) {
			return nil, fmt.Errorf("failed to resolve HEAD: %w", err)
		}
		// unborn branch, headFiles is empty
	} else {
		commitBytes, err := r.Store.Get(headCommitHash)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve HEAD commit %s: %w", headCommitHash, err)
		}
		var commit objects.Commit
		if err := objects.Decode(commitBytes, &commit); err != nil {
			return nil, fmt.Errorf("failed to decode HEAD commit: %w", err)
		}
		treeFiles, err := WalkTree(r.Store, commit.Root)
		if err != nil {
			return nil, fmt.Errorf("failed to walk tree: %w", err)
		}
		for _, tf := range treeFiles {
			headFiles[tf.Path] = tf.Hash
		}
	}

	// 3. Scan Working Directory
	diskFiles := make(map[string]string) // path -> absPath
	err = filepath.WalkDir(r.Root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if absPath == r.TwigDir {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			relPath, err := filepath.Rel(r.Root, absPath)
			if err != nil {
				return err
			}
			normalized := filepath.ToSlash(relPath)
			diskFiles[normalized] = absPath
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to scan working directory: %w", err)
	}

	// 4. Compare and generate status entries
	var entries []StatusEntry

	// We want to process in a deterministic (sorted) order of paths
	allPathsMap := make(map[string]bool)
	for p := range idx.Entries {
		allPathsMap[p] = true
	}
	for p := range diskFiles {
		allPathsMap[p] = true
	}

	var sortedPaths []string
	for p := range allPathsMap {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	for _, p := range sortedPaths {
		idxEntry, inIndex := idx.Get(p)
		absDiskPath, onDisk := diskFiles[p]

		if !inIndex {
			if onDisk {
				entries = append(entries, StatusEntry{Path: p, Status: StatusUntracked})
			}
			continue
		}

		// File is in index
		if idxEntry.Conflict != nil {
			entries = append(entries, StatusEntry{Path: p, Status: StatusConflict})
			continue
		}

		isStagedChange := false
		isWorkdirChange := false

		// Compare Index vs HEAD
		headHash, inHead := headFiles[p]
		if !inHead {
			// In index, not in HEAD -> Staged New
			entries = append(entries, StatusEntry{Path: p, Status: StatusStagedNew})
			isStagedChange = true
		} else {
			if idxEntry.Hash != headHash {
				// In index, in HEAD, different hash -> Staged Modified
				entries = append(entries, StatusEntry{Path: p, Status: StatusStagedModified})
				isStagedChange = true
			}
		}

		// Compare Index vs Disk
		if !onDisk {
			// In index, not on disk -> Deleted
			entries = append(entries, StatusEntry{Path: p, Status: StatusDeleted})
			isWorkdirChange = true
		} else {
			needsRehash, err := index.NeedsRehash(absDiskPath, idxEntry)
			if err != nil {
				return nil, fmt.Errorf("failed to check rehash for %s: %w", p, err)
			}
			if needsRehash {
				computedHash, _, err := ingest.HashFile(absDiskPath)
				if err != nil {
					return nil, fmt.Errorf("failed to compute hash for %s: %w", p, err)
				}
				if computedHash != idxEntry.Hash {
					entries = append(entries, StatusEntry{Path: p, Status: StatusModified})
					isWorkdirChange = true
				}
			}
		}

		// If it's unmodified in both staged and workdir changes, mark as unmodified
		if !isStagedChange && !isWorkdirChange {
			entries = append(entries, StatusEntry{Path: p, Status: StatusUnmodified})
		}
	}

	return entries, nil
}
