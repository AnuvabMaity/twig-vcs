package repo

import (
	"errors"
	"os"
	"path/filepath"

	"twig/internal/objects"
)

// ErrNotARepo is returned when no ".twig" directory is found walking up to the filesystem root.
var ErrNotARepo = errors.New("not a twig repository (or any parent up to root)")

// FindRoot walks upward from startDir (which should be an absolute path)
// looking for a ".twig" directory. It returns the repo root (the directory
// containing .twig) and the full path to .twig itself. If no .twig
// directory is found before reaching the filesystem root, it returns
// ErrNotARepo.
func FindRoot(startDir string) (repoRoot string, twigDir string, err error) {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", "", err
	}

	for {
		candidate := filepath.Join(current, objects.DefaultTwigDir)
		fi, err := os.Stat(candidate)
		if err == nil && fi.IsDir() {
			return current, candidate, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", "", ErrNotARepo
}
