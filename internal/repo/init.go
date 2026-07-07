package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"twig/internal/index"
	"twig/internal/objects"
	"twig/internal/store"
)

// ErrRepoExists is returned when attempting to initialize a repository where a `.twig` directory already exists.
var ErrRepoExists = errors.New("repository already initialized (.twig directory already exists)")

// Init creates a new .twig repository rooted at dir. It creates:
//
//	dir/.twig/objects/            (via store.EnsureLayout)
//	dir/.twig/refs/heads/
//	dir/.twig/HEAD                containing "ref: refs/heads/main\n"
//	dir/.twig/index               an empty, saved Index
//	dir/.twig/config              empty or with sane defaults
//	dir/.twig/VERSION             containing the current objects.FormatVersion
//
// Init returns an error if dir/.twig already exists.
func Init(dir string) error {
	twigDir := filepath.Join(dir, objects.DefaultTwigDir)
	_, err := os.Stat(twigDir)
	if err == nil {
		return ErrRepoExists
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check .twig directory: %w", err)
	}

	// Create objects directory using store.Open + EnsureLayout
	s := store.Open(twigDir)
	if err := s.EnsureLayout(); err != nil {
		return fmt.Errorf("failed to create objects layout: %w", err)
	}

	// Create refs/heads/
	refsHeadsDir := filepath.Join(twigDir, objects.RefsDirName, objects.HeadsDirName)
	if err := os.MkdirAll(refsHeadsDir, objects.DirPermMode); err != nil {
		return fmt.Errorf("failed to create refs/heads directory: %w", err)
	}

	// Create HEAD
	headPath := filepath.Join(twigDir, objects.HeadFileName)
	headContent := "ref: refs/heads/" + objects.DefaultBranchName + "\n"
	if err := os.WriteFile(headPath, []byte(headContent), objects.FilePermMode); err != nil {
		return fmt.Errorf("failed to write HEAD file: %w", err)
	}

	// Create empty index
	indexPath := filepath.Join(twigDir, objects.IndexFileName)
	idx := &index.Index{
		Entries: make(map[string]index.Entry),
	}
	if err := idx.Save(indexPath); err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}

	// Create config
	configPath := filepath.Join(twigDir, objects.ConfigFileName)
	if err := objects.WriteConfig(configPath, make(map[string]string)); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	// Create VERSION
	versionPath := filepath.Join(twigDir, objects.VersionFileName)
	versionContent := strconv.Itoa(objects.FormatVersion) + "\n"
	if err := os.WriteFile(versionPath, []byte(versionContent), objects.FilePermMode); err != nil {
		return fmt.Errorf("failed to write VERSION file: %w", err)
	}

	return nil
}
