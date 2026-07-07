package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"twig/internal/index"
	"twig/internal/ingest"
	"twig/internal/objects"
	"twig/internal/refs"
	"twig/internal/store"
)

// ErrNoCommonAncestor is returned when two commits share no common ancestor in history.
var ErrNoCommonAncestor = errors.New("commits share no common ancestor")

// FindCommonAncestor returns the hash of the nearest commit that both
// hashA and hashB descend from, considering every parent of every
// commit (not just first parents). Returns ErrNoCommonAncestor if none
// exists in this repository's history.
func FindCommonAncestor(s *store.Store, hashA, hashB string) (string, error) {
	ancestorsA, err := getAncestors(s, hashA)
	if err != nil {
		return "", err
	}

	queue := []string{hashB}
	visitedB := map[string]bool{hashB: true}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if ancestorsA[curr] {
			return curr, nil
		}

		commitBytes, err := s.Get(curr)
		if err != nil {
			return "", fmt.Errorf("failed to get commit %s: %w", curr, err)
		}

		var commit objects.Commit
		if err := objects.Decode(commitBytes, &commit); err != nil {
			return "", fmt.Errorf("failed to decode commit %s: %w", curr, err)
		}

		for _, p := range commit.Parents {
			if !visitedB[p] {
				visitedB[p] = true
				queue = append(queue, p)
			}
		}
	}

	return "", ErrNoCommonAncestor
}

// getAncestors walks all ancestors of startHash (including startHash itself)
// and returns them as a set of hashes.
func getAncestors(s *store.Store, startHash string) (map[string]bool, error) {
	ancestors := make(map[string]bool)
	queue := []string{startHash}
	ancestors[startHash] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		commitBytes, err := s.Get(curr)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit %s: %w", curr, err)
		}

		var commit objects.Commit
		if err := objects.Decode(commitBytes, &commit); err != nil {
			return nil, fmt.Errorf("failed to decode commit %s: %w", curr, err)
		}

		for _, p := range commit.Parents {
			if !ancestors[p] {
				ancestors[p] = true
				queue = append(queue, p)
			}
		}
	}
	return ancestors, nil
}

// DiffStatus represents the category of change between two tree versions of a file.
type DiffStatus string

const (
	DiffAdded     DiffStatus = "added"
	DiffRemoved   DiffStatus = "removed"
	DiffChanged   DiffStatus = "changed"
	DiffUnchanged DiffStatus = "unchanged"
)

// DiffEntry records the path, status, hash, and object type for a path compared across trees.
type DiffEntry struct {
	Path   string
	Status DiffStatus
	Hash   string // the hash on the "new" side; empty if Status is DiffRemoved
	Type   objects.ObjectType
}

// DiffTrees compares the files resolved (via WalkTree) from oldTreeHash
// against newTreeHash and returns one DiffEntry per path that differs.
// Paths identical in both trees are included only if includeUnchanged
// is true (the merge logic in PH7-T04 needs to see unchanged paths too,
// to tell "changed on neither side" apart from "changed on exactly one
// side" — plain diff display does not).
func DiffTrees(s *store.Store, oldTreeHash, newTreeHash string, includeUnchanged bool) ([]DiffEntry, error) {
	var oldFiles []TreeFile
	var err error
	if oldTreeHash != "" {
		oldFiles, err = WalkTree(s, oldTreeHash)
		if err != nil {
			return nil, fmt.Errorf("failed to walk old tree: %w", err)
		}
	}

	var newFiles []TreeFile
	if newTreeHash != "" {
		newFiles, err = WalkTree(s, newTreeHash)
		if err != nil {
			return nil, fmt.Errorf("failed to walk new tree: %w", err)
		}
	}

	oldMap := make(map[string]TreeFile)
	for _, f := range oldFiles {
		oldMap[f.Path] = f
	}

	newMap := make(map[string]TreeFile)
	for _, f := range newFiles {
		newMap[f.Path] = f
	}

	allPathsMap := make(map[string]bool)
	for p := range oldMap {
		allPathsMap[p] = true
	}
	for p := range newMap {
		allPathsMap[p] = true
	}

	var sortedPaths []string
	for p := range allPathsMap {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	var diffs []DiffEntry

	for _, p := range sortedPaths {
		oldFile, inOld := oldMap[p]
		newFile, inNew := newMap[p]

		if !inOld && inNew {
			diffs = append(diffs, DiffEntry{
				Path:   p,
				Status: DiffAdded,
				Hash:   newFile.Hash,
				Type:   newFile.Type,
			})
		} else if inOld && !inNew {
			diffs = append(diffs, DiffEntry{
				Path:   p,
				Status: DiffRemoved,
				Hash:   "",
				Type:   oldFile.Type,
			})
		} else if inOld && inNew {
			if oldFile.Hash != newFile.Hash {
				diffs = append(diffs, DiffEntry{
					Path:   p,
					Status: DiffChanged,
					Hash:   newFile.Hash,
					Type:   newFile.Type,
				})
			} else if includeUnchanged {
				diffs = append(diffs, DiffEntry{
					Path:   p,
					Status: DiffUnchanged,
					Hash:   newFile.Hash,
					Type:   newFile.Type,
				})
			}
		}
	}

	return diffs, nil
}

// ErrMergeConflicts is returned when a merge has unresolved conflicts.
var ErrMergeConflicts = errors.New("merge has unresolved conflicts, resolve them before committing")

// MergeConflictsError holds the list of conflicting paths.
type MergeConflictsError struct {
	Conflicts []string
}

func (e *MergeConflictsError) Error() string {
	return ErrMergeConflicts.Error()
}

func (e *MergeConflictsError) Is(target error) bool {
	return target == ErrMergeConflicts
}

// Merge merges the named branch into the currently checked-out branch.
func (r *Repo) Merge(branchName string) error {
	// 1. Load staging index
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	idx, err := index.Load(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	// 2. Resolve target branch
	targetCommitHash, err := refs.ReadBranch(r.TwigDir, branchName)
	if err != nil {
		return fmt.Errorf("failed to resolve target branch %s: %w", branchName, err)
	}

	// 3. Resolve current HEAD
	headCommitHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		return fmt.Errorf("failed to resolve HEAD: %w", err)
	}

	// 4. Find common ancestor
	ancestorCommitHash, err := FindCommonAncestor(r.Store, headCommitHash, targetCommitHash)
	if err != nil {
		return fmt.Errorf("failed to find common ancestor: %w", err)
	}

	// If target branch is already fully merged
	if ancestorCommitHash == targetCommitHash {
		return nil
	}

	// 5. Walk trees
	ancestorTreeHash, err := getTreeHash(r.Store, ancestorCommitHash)
	if err != nil {
		return err
	}
	oursTreeHash, err := getTreeHash(r.Store, headCommitHash)
	if err != nil {
		return err
	}
	theirsTreeHash, err := getTreeHash(r.Store, targetCommitHash)
	if err != nil {
		return err
	}

	ancestorFilesList, err := WalkTree(r.Store, ancestorTreeHash)
	if err != nil {
		return err
	}
	oursFilesList, err := WalkTree(r.Store, oursTreeHash)
	if err != nil {
		return err
	}
	theirsFilesList, err := WalkTree(r.Store, theirsTreeHash)
	if err != nil {
		return err
	}

	ancestorFiles := make(map[string]TreeFile)
	for _, f := range ancestorFilesList {
		ancestorFiles[f.Path] = f
	}
	oursFiles := make(map[string]TreeFile)
	for _, f := range oursFilesList {
		oursFiles[f.Path] = f
	}
	theirsFiles := make(map[string]TreeFile)
	for _, f := range theirsFilesList {
		theirsFiles[f.Path] = f
	}

	// Collect unique paths
	allPathsMap := make(map[string]bool)
	for p := range ancestorFiles {
		allPathsMap[p] = true
	}
	for p := range oursFiles {
		allPathsMap[p] = true
	}
	for p := range theirsFiles {
		allPathsMap[p] = true
	}

	var allPaths []string
	for p := range allPathsMap {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)

	hasConflicts := false
	var conflicts []string

	for _, p := range allPaths {
		fAnc, inAnc := ancestorFiles[p]
		fOurs, inOurs := oursFiles[p]
		fTheirs, inTheirs := theirsFiles[p]

		oursChanged := false
		oursDeleted := false
		if inAnc {
			if !inOurs {
				oursChanged = true
				oursDeleted = true
			} else if fOurs.Hash != fAnc.Hash {
				oursChanged = true
			}
		} else {
			if inOurs {
				oursChanged = true
			}
		}

		theirsChanged := false
		theirsDeleted := false
		if inAnc {
			if !inTheirs {
				theirsChanged = true
				theirsDeleted = true
			} else if fTheirs.Hash != fAnc.Hash {
				theirsChanged = true
			}
		} else {
			if inTheirs {
				theirsChanged = true
			}
		}

		absPath := filepath.Join(r.Root, filepath.FromSlash(p))

		// 3-way Merge Decisions
		if !oursChanged && !theirsChanged {
			continue
		}

		if oursChanged && !theirsChanged {
			continue
		}

		if !oursChanged && theirsChanged {
			if theirsDeleted {
				_ = os.Remove(absPath)
				idx.Remove(p)
				cleanEmptyParents(r.Root, absPath)
			} else {
				size, mtime, err := reconstructFile(r.Store, absPath, fTheirs.Hash, fTheirs.Type)
				if err != nil {
					return fmt.Errorf("failed to reconstruct file %s: %w", p, err)
				}
				idx.Put(p, index.Entry{
					Hash:    fTheirs.Hash,
					Type:    fTheirs.Type,
					Size:    size,
					ModTime: mtime,
				})
			}
			continue
		}

		// Changed on both sides
		bothDeleted := oursDeleted && theirsDeleted
		bothAddedOrModifiedIdentical := !oursDeleted && !theirsDeleted && inOurs && inTheirs && fOurs.Hash == fTheirs.Hash

		if bothDeleted || bothAddedOrModifiedIdentical {
			continue
		}

		// Conflict!
		hasConflicts = true
		conflicts = append(conflicts, p)

		var oursHash string
		var oursType objects.ObjectType
		if inOurs {
			oursHash = fOurs.Hash
			oursType = fOurs.Type
		}

		var theirsHash string
		var theirsType objects.ObjectType
		if inTheirs {
			theirsHash = fTheirs.Hash
			theirsType = fTheirs.Type
		}

		entry, exists := idx.Get(p)
		if !exists {
			entry = index.Entry{
				Hash: oursHash,
				Type: oursType,
			}
			if inOurs {
				if fi, err := os.Stat(absPath); err == nil {
					entry.Size = fi.Size()
					entry.ModTime = fi.ModTime().UnixNano()
				}
			}
		}

		entry.Conflict = &index.Conflict{
			OursHash:   oursHash,
			OursType:   oursType,
			TheirsHash: theirsHash,
			TheirsType: theirsType,
		}
		idx.Put(p, entry)
	}

	// Save the index to disk regardless of conflict status
	if err := idx.Save(indexPath); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	if hasConflicts {
		sort.Strings(conflicts)
		mergeHeadPath := filepath.Join(r.TwigDir, "MERGE_HEAD")
		if err := os.WriteFile(mergeHeadPath, []byte(targetCommitHash+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write MERGE_HEAD: %w", err)
		}
		return &MergeConflictsError{Conflicts: conflicts}
	}

	// No conflicts! Create merge commit
	rootTreeHash, err := BuildTree(r.Store, idx.Entries)
	if err != nil {
		return fmt.Errorf("failed to build merge tree: %w", err)
	}

	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	authorID, err := objects.ResolveAuthorID(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve author identity: %w", err)
	}

	message := fmt.Sprintf("Merge branch '%s'", branchName)
	parents := []string{headCommitHash, targetCommitHash}

	commitHash, err := BuildCommit(r.Store, rootTreeHash, parents, authorID, message)
	if err != nil {
		return fmt.Errorf("failed to build merge commit: %w", err)
	}

	// Update branch reference
	target, isBranch, err := refs.ReadHEAD(r.TwigDir)
	if err != nil {
		return fmt.Errorf("failed to read HEAD state: %w", err)
	}

	if isBranch {
		if err := refs.WriteBranch(r.TwigDir, target, commitHash); err != nil {
			return fmt.Errorf("failed to update branch ref %s: %w", target, err)
		}
	} else {
		if err := refs.WriteHEADDetached(r.TwigDir, commitHash); err != nil {
			return fmt.Errorf("failed to update detached HEAD: %w", err)
		}
	}

	return nil
}

func getTreeHash(s *store.Store, commitHash string) (string, error) {
	commitBytes, err := s.Get(commitHash)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve commit %s: %w", commitHash, err)
	}
	var commit objects.Commit
	if err := objects.Decode(commitBytes, &commit); err != nil {
		return "", fmt.Errorf("failed to decode commit %s: %w", commitHash, err)
	}
	return commit.Root, nil
}

func cleanEmptyParents(root string, path string) {
	parentDir := filepath.Dir(path)
	for parentDir != root {
		if err := os.Remove(parentDir); err != nil {
			break
		}
		parentDir = filepath.Dir(parentDir)
	}
}

func reconstructFile(s *store.Store, absPath string, hash string, objType objects.ObjectType) (size int64, mtime int64, err error) {
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	tempFile, err := os.CreateTemp(dir, "reconstruct-*")
	if err != nil {
		return 0, 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempName := tempFile.Name()
	var tempClosed bool
	var success bool
	defer func() {
		if !success {
			if !tempClosed {
				tempFile.Close()
			}
			os.Remove(tempName)
		}
	}()

	if err := ingest.Reconstruct(s, hash, objType, tempFile); err != nil {
		return 0, 0, fmt.Errorf("failed to reconstruct content: %w", err)
	}

	if err := tempFile.Sync(); err != nil {
		return 0, 0, fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		return 0, 0, fmt.Errorf("failed to close temp file: %w", err)
	}
	tempClosed = true

	if err := os.Rename(tempName, absPath); err != nil {
		return 0, 0, fmt.Errorf("failed to rename temp file to %s: %w", absPath, err)
	}
	success = true

	fi, err := os.Stat(absPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to stat reconstructed file %s: %w", absPath, err)
	}

	return fi.Size(), fi.ModTime().UnixNano(), nil
}

// ErrNoConflict is returned when attempting to resolve a path that has no conflict in the index.
var ErrNoConflict = errors.New("no conflict on path")

// ResolveConflict resolves a conflict on the given path in favor of either "ours" or "theirs".
func (r *Repo) ResolveConflict(path string, side string) error {
	if side != "ours" && side != "theirs" {
		return fmt.Errorf("invalid side %q: must be 'ours' or 'theirs'", side)
	}

	absWDPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	relPath, err := filepath.Rel(r.Root, absWDPath)
	if err != nil {
		return fmt.Errorf("path is outside repository root: %w", err)
	}
	normalizedPath := filepath.ToSlash(relPath)

	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	idx, err := index.Load(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	entry, exists := idx.Get(normalizedPath)
	if !exists || entry.Conflict == nil {
		return ErrNoConflict
	}

	var chosenHash string
	var chosenType objects.ObjectType
	if side == "ours" {
		chosenHash = entry.Conflict.OursHash
		chosenType = entry.Conflict.OursType
	} else {
		chosenHash = entry.Conflict.TheirsHash
		chosenType = entry.Conflict.TheirsType
	}

	if chosenHash == "" {
		// Side deleted the file: remove from working directory and index
		_ = os.Remove(absWDPath)
		cleanEmptyParents(r.Root, absWDPath)
		idx.Remove(normalizedPath)
	} else if side == "ours" {
		// Ours: preserve working copy, just clear Conflict field and stat current file
		entry.Hash = chosenHash
		entry.Type = chosenType
		entry.Conflict = nil
		if fi, err := os.Stat(absWDPath); err == nil {
			entry.Size = fi.Size()
			entry.ModTime = fi.ModTime().UnixNano()
		}
		idx.Put(normalizedPath, entry)
	} else {
		// Theirs: overwrite working copy, clear Conflict field
		size, mtime, err := reconstructFile(r.Store, absWDPath, chosenHash, chosenType)
		if err != nil {
			return fmt.Errorf("failed to write resolved file content: %w", err)
		}
		entry.Hash = chosenHash
		entry.Type = chosenType
		entry.Size = size
		entry.ModTime = mtime
		entry.Conflict = nil
		idx.Put(normalizedPath, entry)
	}

	if err := idx.Save(indexPath); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}
