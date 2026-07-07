package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"twig/internal/index"
	"twig/internal/ingest"
	"twig/internal/objects"
	"twig/internal/refs"
	"twig/internal/store"
)

// Repo represents a Twig repository.
type Repo struct {
	Root    string
	TwigDir string
	Store   *store.Store
}

// Open discovers the repo containing startDir and returns a Repo with
// its Store ready to use.
func Open(startDir string) (*Repo, error) {
	root, twigDir, err := FindRoot(startDir)
	if err != nil {
		return nil, err
	}

	st := store.Open(twigDir)
	return &Repo{
		Root:    root,
		TwigDir: twigDir,
		Store:   st,
	}, nil
}

// AddFile stages a file or directory. If the path is a directory, it walks it recursively,
// staging all regular files under it, skipping the .twig repository directory itself.
func (r *Repo) AddFile(relOrAbsPath string) error {
	absPath, err := filepath.Abs(relOrAbsPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	absRoot, err := filepath.Abs(r.Root)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of root: %w", err)
	}

	relPath, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return fmt.Errorf("path %s is not under repository root %s: %w", relOrAbsPath, r.Root, err)
	}

	// Verify that the file/directory is not outside the repository root directory.
	if strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
		return fmt.Errorf("path %s is outside repository root %s", relOrAbsPath, r.Root)
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat: %w", err)
	}

	indexPath := filepath.Join(r.TwigDir, "index")
	idx, err := index.Load(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	if !fi.IsDir() {
		normalizedPath := filepath.ToSlash(relPath)
		if err := r.addSingleFile(idx, absPath, normalizedPath, fi); err != nil {
			return err
		}
	} else {
		// Recursive directory walk. Note: A .twigignore exclusion file was descoped as a stretch goal.
		err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			absItemPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}

			if d.IsDir() {
				if absItemPath == r.TwigDir {
					return filepath.SkipDir
				}
				return nil
			}

			if d.Type().IsRegular() {
				itemRelPath, err := filepath.Rel(absRoot, absItemPath)
				if err != nil {
					return err
				}
				normalizedItemPath := filepath.ToSlash(itemRelPath)

				info, err := d.Info()
				if err != nil {
					return err
				}

				if err := r.addSingleFile(idx, absItemPath, normalizedItemPath, info); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to walk directory: %w", err)
		}
	}

	if err := idx.Save(indexPath); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

// addSingleFile ingests a single file's content and records its entry in the index.
func (r *Repo) addSingleFile(idx *index.Index, absPath string, normalizedPath string, fi os.FileInfo) error {
	hash, objType, err := ingest.IngestFile(r.Store, absPath)
	if err != nil {
		return fmt.Errorf("failed to ingest file %s: %w", absPath, err)
	}

	idx.Put(normalizedPath, index.Entry{
		Hash:    hash,
		Type:    objType,
		Size:    fi.Size(),
		ModTime: fi.ModTime().UnixNano(),
	})
	return nil
}

// ErrNothingToCommit is returned when there are no changes staged since the last commit.
var ErrNothingToCommit = errors.New("nothing to commit")

// Commit builds a Tree from the repo's current index, builds a Commit
// object whose parent is the current HEAD commit (if any — none for the
// first commit), and updates the current branch ref to point at the new
// commit. Returns ErrNothingToCommit if the resulting root tree hash is
// identical to HEAD's current tree hash (i.e., nothing staged has
// changed since the last commit).
func (r *Repo) Commit(message string) (commitHash string, err error) {
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	idx, err := index.Load(indexPath)
	if err != nil {
		return "", fmt.Errorf("failed to load index: %w", err)
	}

	// Block commit if there are unresolved conflicts
	for _, entry := range idx.Entries {
		if entry.Conflict != nil {
			return "", ErrMergeConflicts
		}
	}

	rootTreeHash, err := BuildTree(r.Store, idx.Entries)
	if err != nil {
		return "", fmt.Errorf("failed to build tree: %w", err)
	}

	var parents []string
	headCommitHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		if !errors.Is(err, refs.ErrUnbornBranch) {
			return "", fmt.Errorf("failed to resolve HEAD: %w", err)
		}
	} else {
		parents = []string{headCommitHash}
	}

	// Read second parent if MERGE_HEAD exists
	mergeHeadPath := filepath.Join(r.TwigDir, "MERGE_HEAD")
	if mergeHeadBytes, err := os.ReadFile(mergeHeadPath); err == nil {
		mergeHead := strings.TrimSpace(string(mergeHeadBytes))
		if mergeHead != "" {
			parents = append(parents, mergeHead)
		}
	}

	if len(parents) > 0 {
		parentCommitBytes, err := r.Store.Get(headCommitHash)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve parent commit: %w", err)
		}

		var parentCommit objects.Commit
		if err := objects.Decode(parentCommitBytes, &parentCommit); err != nil {
			return "", fmt.Errorf("failed to decode parent commit: %w", err)
		}

		if len(parents) == 1 && parentCommit.Root == rootTreeHash {
			return "", ErrNothingToCommit
		}
	}

	configPath := filepath.Join(r.TwigDir, objects.ConfigFileName)
	authorID, err := objects.ResolveAuthorID(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve committer identity: %w", err)
	}

	commitHash, err = BuildCommit(r.Store, rootTreeHash, parents, authorID, message)
	if err != nil {
		return "", fmt.Errorf("failed to build commit object: %w", err)
	}

	target, isBranch, err := refs.ReadHEAD(r.TwigDir)
	if err != nil {
		return "", fmt.Errorf("failed to read HEAD state: %w", err)
	}

	if isBranch {
		if err := refs.WriteBranch(r.TwigDir, target, commitHash); err != nil {
			return "", fmt.Errorf("failed to update branch ref %s: %w", target, err)
		}
	} else {
		if err := refs.WriteHEADDetached(r.TwigDir, commitHash); err != nil {
			return "", fmt.Errorf("failed to update detached HEAD: %w", err)
		}
	}

	// Clean up MERGE_HEAD
	_ = os.Remove(mergeHeadPath)

	return commitHash, nil
}

// ErrWouldOverwriteChanges is returned when checkout would overwrite uncommitted local changes.
var ErrWouldOverwriteChanges = errors.New("checkout would overwrite local changes")

// Checkout resolves ref (a branch name, checked first, or else treated
// as a raw commit hash) to a commit, walks its tree, and writes the
// result into the repo's working directory. Before writing, for every
// target file that already exists on disk, Checkout re-ingests it
// (cheaply) and compares the resulting hash to what the index currently
// has recorded for that path. If any file's on-disk content differs from
// what the index last recorded, Checkout returns ErrWouldOverwriteChanges
// listing the conflicting paths, unless force is true. On success, Checkout
// updates HEAD: symbolically if ref matched a branch name, detached otherwise,
// and also updates the index to match the new tree.
func (r *Repo) Checkout(ref string, force bool) error {
	// 1. Resolve ref to a commit hash
	commitHash, err := refs.ReadBranch(r.TwigDir, ref)
	isBranch := true
	if err != nil {
		commitHash = ref
		isBranch = false
	}

	// Fast-path: check if we are already on this branch or detached commit
	currTarget, currIsBranch, err := refs.ReadHEAD(r.TwigDir)
	if err == nil {
		if currIsBranch && isBranch && currTarget == ref {
			return nil
		}
		if !currIsBranch && !isBranch && currTarget == commitHash {
			return nil
		}
	}

	// 2. Retrieve commit from store
	commitBytes, err := r.Store.Get(commitHash)
	if err != nil {
		return fmt.Errorf("commit %s not found: %w", commitHash, err)
	}

	var commit objects.Commit
	if err := objects.Decode(commitBytes, &commit); err != nil {
		return fmt.Errorf("failed to decode commit %s: %w", commitHash, err)
	}

	// 3. Walk the target commit tree
	files, err := WalkTree(r.Store, commit.Root)
	if err != nil {
		return fmt.Errorf("failed to walk tree: %w", err)
	}

	// 4. Load current index
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	idx, err := index.Load(indexPath)
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	// 5. Check for conflicts if force is false
	if !force {
		var conflicts []string
		for _, tf := range files {
			targetPath := filepath.Join(r.Root, filepath.FromSlash(tf.Path))
			if _, err := os.Stat(targetPath); err == nil {
				// Re-ingest the on-disk file
				hash, _, err := ingest.IngestFile(r.Store, targetPath)
				if err != nil {
					return fmt.Errorf("failed to check on-disk file %s: %w", tf.Path, err)
				}

				entry, ok := idx.Get(tf.Path)
				if !ok || entry.Hash != hash {
					conflicts = append(conflicts, tf.Path)
				}
			}
		}

		if len(conflicts) > 0 {
			return fmt.Errorf("%w: %v", ErrWouldOverwriteChanges, conflicts)
		}
	}

	// 6. Write working directory
	if err := WriteWorkingDir(r.Store, r.Root, files); err != nil {
		return fmt.Errorf("failed to write working directory: %w", err)
	}

	// 7. Clean up files that are in the current index but not in the new tree
	newFilesMap := make(map[string]bool)
	for _, tf := range files {
		newFilesMap[tf.Path] = true
	}

	for path := range idx.Entries {
		if !newFilesMap[path] {
			targetPath := filepath.Join(r.Root, filepath.FromSlash(path))
			_ = os.Remove(targetPath)

			// Clean up parent directory if empty
			parentDir := filepath.Dir(targetPath)
			for parentDir != r.Root {
				if err := os.Remove(parentDir); err != nil {
					break
				}
				parentDir = filepath.Dir(parentDir)
			}
		}
	}

	// 8. Update index to match the new tree exactly
	newIdx := &index.Index{
		Entries: make(map[string]index.Entry),
	}
	for _, tf := range files {
		targetPath := filepath.Join(r.Root, filepath.FromSlash(tf.Path))
		fi, err := os.Stat(targetPath)
		if err != nil {
			return fmt.Errorf("failed to stat written file %s: %w", tf.Path, err)
		}

		newIdx.Put(tf.Path, index.Entry{
			Hash:    tf.Hash,
			Type:    tf.Type,
			Size:    fi.Size(),
			ModTime: fi.ModTime().UnixNano(),
		})
	}

	if err := newIdx.Save(indexPath); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	// 9. Update HEAD reference
	if isBranch {
		if err := refs.WriteHEAD(r.TwigDir, ref); err != nil {
			return fmt.Errorf("failed to update HEAD symbolic ref: %w", err)
		}
	} else {
		if err := refs.WriteHEADDetached(r.TwigDir, commitHash); err != nil {
			return fmt.Errorf("failed to update HEAD detached: %w", err)
		}
	}

	// Clean up MERGE_HEAD since we successfully checked out a branch/commit (merge aborted/completed)
	_ = os.Remove(filepath.Join(r.TwigDir, "MERGE_HEAD"))

	return nil
}

// ErrBranchExists is returned when attempting to create a branch that already exists.
var ErrBranchExists = errors.New("branch already exists")

// CreateBranch creates a new branch ref named name, pointing at the
// current HEAD commit. Returns refs.ErrUnbornBranch if there is no
// commit yet to point at, and ErrBranchExists if a branch with this
// name already exists.
func (r *Repo) CreateBranch(name string) error {
	_, err := refs.ReadBranch(r.TwigDir, name)
	if err == nil {
		return ErrBranchExists
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check existing branch: %w", err)
	}
	commitHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		return err // e.g. refs.ErrUnbornBranch
	}
	if err := refs.WriteBranch(r.TwigDir, name, commitHash); err != nil {
		return fmt.Errorf("failed to write new branch: %w", err)
	}
	return nil
}
