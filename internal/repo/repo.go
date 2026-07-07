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

	if len(parents) > 0 {
		parentCommitBytes, err := r.Store.Get(headCommitHash)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve parent commit: %w", err)
		}

		var parentCommit objects.Commit
		if err := objects.Decode(parentCommitBytes, &parentCommit); err != nil {
			return "", fmt.Errorf("failed to decode parent commit: %w", err)
		}

		if parentCommit.Root == rootTreeHash {
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

	return commitHash, nil
}

