package repo

import (
	"fmt"

	"twig/internal/objects"
)

// LogEntry bundles a commit hash and its decoded Commit object.
type LogEntry struct {
	Hash   string
	Commit objects.Commit
}

// Log walks the commit parent chain starting at startHash (typically
// the result of refs.ResolveHEAD) and returns entries newest-first.
// If a commit has multiple parents (future merge commits), Log follows
// only the first parent (Commit.Parents[0]) to traverse linear history.
// If startHash is "", Log returns (nil, nil) (unborn branch state).
func (r *Repo) Log(startHash string) ([]LogEntry, error) {
	if startHash == "" {
		return nil, nil
	}

	var entries []LogEntry
	currentHash := startHash

	for currentHash != "" {
		data, err := r.Store.Get(currentHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit %s: %w", currentHash, err)
		}

		var commit objects.Commit
		if err := objects.Decode(data, &commit); err != nil {
			return nil, fmt.Errorf("failed to decode commit %s: %w", currentHash, err)
		}

		entries = append(entries, LogEntry{
			Hash:   currentHash,
			Commit: commit,
		})

		// For MVP, follow only the first parent to traverse linear history
		if len(commit.Parents) > 0 {
			currentHash = commit.Parents[0]
		} else {
			currentHash = ""
		}
	}

	return entries, nil
}
