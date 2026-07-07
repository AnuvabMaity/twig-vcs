package refs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"twig/internal/objects"
)

// ErrUnbornBranch is returned when the HEAD points to a branch that has no commits yet.
var ErrUnbornBranch = errors.New("branch has no commits yet")

const (
	headFileName = objects.HeadFileName
	refsDirName  = objects.RefsDirName
	headsDirName = objects.HeadsDirName
	dirPermMode  = objects.DirPermMode
	filePermMode = objects.FilePermMode
)

// ReadHEAD returns what HEAD currently points to. If HEAD is a symbolic
// ref ("ref: refs/heads/<name>"), isBranch is true and target is <name>.
// If HEAD contains a raw commit hash (detached), isBranch is false and
// target is that hash.
func ReadHEAD(twigDir string) (target string, isBranch bool, err error) {
	headPath := filepath.Join(twigDir, headFileName)
	contentBytes, err := os.ReadFile(headPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to read HEAD file: %w", err)
	}

	content := strings.TrimSpace(string(contentBytes))
	prefix := "ref: refs/heads/"
	if after, ok := strings.CutPrefix(content, prefix); ok {
		branchName := after
		return branchName, true, nil
	}

	return content, false, nil
}

// WriteHEAD points HEAD symbolically at the given branch name.
func WriteHEAD(twigDir, branchName string) error {
	headPath := filepath.Join(twigDir, headFileName)
	content := fmt.Sprintf("ref: %s/%s/%s\n", refsDirName, headsDirName, branchName)
	if err := os.WriteFile(headPath, []byte(content), filePermMode); err != nil {
		return fmt.Errorf("failed to write HEAD file: %w", err)
	}
	return nil
}

// WriteHEADDetached points HEAD directly at a commit hash.
func WriteHEADDetached(twigDir, commitHash string) error {
	headPath := filepath.Join(twigDir, headFileName)
	content := fmt.Sprintf("%s\n", commitHash)
	if err := os.WriteFile(headPath, []byte(content), filePermMode); err != nil {
		return fmt.Errorf("failed to write HEAD file (detached): %w", err)
	}
	return nil
}

// ResolveHEAD returns the commit hash HEAD currently resolves to,
// following the branch ref if HEAD is symbolic. If HEAD points at a
// branch that exists but has no commits yet (a brand-new repo),
// ResolveHEAD returns ("", ErrUnbornBranch) — this is an expected,
// normal state, not a failure the caller should treat as fatal.
func ResolveHEAD(twigDir string) (commitHash string, err error) {
	target, isBranch, err := ReadHEAD(twigDir)
	if err != nil {
		return "", err
	}

	if !isBranch {
		return target, nil
	}

	commitHash, err = ReadBranch(twigDir, target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			return "", ErrUnbornBranch
		}
		return "", err
	}

	return commitHash, nil
}

// ReadBranch returns the commit hash the named branch ref points to.
func ReadBranch(twigDir, name string) (commitHash string, err error) {
	branchPath := filepath.Join(twigDir, refsDirName, headsDirName, name)
	contentBytes, err := os.ReadFile(branchPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(contentBytes)), nil
}

// WriteBranch creates or updates a branch ref to point at commitHash.
func WriteBranch(twigDir, name, commitHash string) error {
	branchPath := filepath.Join(twigDir, refsDirName, headsDirName, name)
	dir := filepath.Dir(branchPath)
	if err := os.MkdirAll(dir, dirPermMode); err != nil {
		return fmt.Errorf("failed to create branch ref directories: %w", err)
	}

	content := fmt.Sprintf("%s\n", commitHash)
	if err := os.WriteFile(branchPath, []byte(content), filePermMode); err != nil {
		return fmt.Errorf("failed to write branch ref file: %w", err)
	}
	return nil
}

// ListBranches returns the names of every existing branch (i.e. every
// file under refs/heads/), in no particular guaranteed order.
func ListBranches(twigDir string) ([]string, error) {
	headsDir := filepath.Join(twigDir, refsDirName, headsDirName)
	var branches []string
	err := filepath.WalkDir(headsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Calculate the relative path from headsDir to get the branch name
		rel, err := filepath.Rel(headsDir, path)
		if err != nil {
			return err
		}
		// Convert separators to forward slashes for branch names with path components
		branches = append(branches, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}
	return branches, nil
}
