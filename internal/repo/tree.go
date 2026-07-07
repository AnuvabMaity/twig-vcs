package repo

import (
	"strings"

	"twig/internal/index"
	"twig/internal/objects"
	"twig/internal/store"
)

// BuildTree groups the given index entries (path -> index.Entry) by
// directory, recursively builds and stores a Tree object for every
// directory level (deepest first), and returns the hash of the root
// Tree. Entries within each Tree are sorted by name via
// objects.SortTreeEntries before encoding, so identical directory
// contents always produce an identical Tree hash.
func BuildTree(s *store.Store, entries map[string]index.Entry) (rootHash string, err error) {
	return buildTreeRecursive(s, entries)
}

func buildTreeRecursive(s *store.Store, entries map[string]index.Entry) (string, error) {
	if len(entries) == 0 {
		treeObj := objects.Tree{
			Type:    objects.TypeTree,
			Entries: []objects.TreeEntry{},
		}
		encoded, err := objects.Encode(treeObj)
		if err != nil {
			return "", err
		}
		return s.Put(encoded)
	}

	leaves := make(map[string]index.Entry)
	subDirs := make(map[string]map[string]index.Entry)

	for path, entry := range entries {
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 1 {
			leaves[parts[0]] = entry
		} else {
			dirName := parts[0]
			subPath := parts[1]
			if _, ok := subDirs[dirName]; !ok {
				subDirs[dirName] = make(map[string]index.Entry)
			}
			subDirs[dirName][subPath] = entry
		}
	}

	var treeEntries []objects.TreeEntry

	for name, entry := range leaves {
		treeEntries = append(treeEntries, objects.TreeEntry{
			Name: name,
			Hash: entry.Hash,
			Type: entry.Type,
		})
	}

	for dirName, subEntries := range subDirs {
		subTreeHash, err := buildTreeRecursive(s, subEntries)
		if err != nil {
			return "", err
		}
		treeEntries = append(treeEntries, objects.TreeEntry{
			Name: dirName,
			Hash: subTreeHash,
			Type: objects.TypeTree,
		})
	}

	objects.SortTreeEntries(treeEntries)

	treeObj := objects.Tree{
		Type:    objects.TypeTree,
		Entries: treeEntries,
	}

	encoded, err := objects.Encode(treeObj)
	if err != nil {
		return "", err
	}

	return s.Put(encoded)
}
