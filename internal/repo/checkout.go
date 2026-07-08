package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"twig/internal/ingest"
	"twig/internal/objects"
	"twig/internal/store"
)

// TreeFile is one resolved file within a walked tree.
type TreeFile struct {
	Path string // relative to repo root, forward-slash separated
	Hash string
	Type objects.ObjectType // TypeBlob or TypeAsset (never TypeTree — those are traversed, not returned)
}

// WalkTree recursively resolves the Tree object at treeHash into a flat
// list of every file it (transitively) contains.
func WalkTree(s *store.Store, treeHash string) ([]TreeFile, error) {
	return walkTreeRecursive(s, treeHash, "")
}

func walkTreeRecursive(s *store.Store, treeHash string, prefix string) ([]TreeFile, error) {
	data, err := s.Get(treeHash)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tree %s: %w", treeHash, err)
	}

	var tree objects.Tree
	if err := objects.Decode(data, &tree); err != nil {
		return nil, fmt.Errorf("failed to decode tree %s: %w", treeHash, err)
	}

	var files []TreeFile
	for _, entry := range tree.Entries {
		if entry.Name == "" || entry.Name == "." || entry.Name == ".." || strings.Contains(entry.Name, "/") || strings.Contains(entry.Name, "\\") {
			return nil, fmt.Errorf("malicious or invalid entry name in tree object: %q", entry.Name)
		}

		entryPath := entry.Name
		if prefix != "" {
			entryPath = prefix + "/" + entry.Name
		}

		switch entry.Type {
		case objects.TypeBlob, objects.TypeAsset:
			files = append(files, TreeFile{
				Path: entryPath,
				Hash: entry.Hash,
				Type: entry.Type,
			})
		case objects.TypeTree:
			subFiles, err := walkTreeRecursive(s, entry.Hash, entryPath)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		default:
			return nil, fmt.Errorf("unknown object type %s for entry %s in tree %s", entry.Type, entry.Name, treeHash)
		}
	}

	return files, nil
}

// WriteWorkingDir writes every file in files to disk rooted at root,
// creating parent directories as needed, using ingest.Reconstruct to
// resolve each file's content from the object store.
func WriteWorkingDir(s *store.Store, root string, files []TreeFile) error {
	for _, tf := range files {
		targetPath := filepath.Join(root, filepath.FromSlash(tf.Path))
		rel, err := filepath.Rel(root, targetPath)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return fmt.Errorf("security violation: path %s escapes repository root", tf.Path)
		}

		dir := filepath.Dir(targetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directories for %s: %w", tf.Path, err)
		}

		f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", tf.Path, err)
		}

		err = ingest.Reconstruct(s, tf.Hash, tf.Type, f)
		closeErr := f.Close()
		if err != nil {
			return fmt.Errorf("failed to reconstruct file %s: %w", tf.Path, err)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close file %s: %w", tf.Path, closeErr)
		}
	}
	return nil
}
