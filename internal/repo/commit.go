package repo

import (
	"time"

	"twig/internal/objects"
	"twig/internal/store"
)

// BuildCommit constructs and stores a Commit object referencing rootHash
// as its tree, parents as its parent commit hashes (empty for the very
// first commit), authorID as the committer, and message as the commit
// message. The commit's Author.Time is set to time.Now().Unix().
// Returns the hash of the stored Commit object.
func BuildCommit(s *store.Store, rootHash string, parents []string, authorID, message string) (commitHash string, err error) {
	if parents == nil {
		parents = []string{}
	}

	commit := objects.Commit{
		Type:    objects.TypeCommit,
		Root:    rootHash,
		Parents: parents,
		Author: objects.Author{
			ID:   authorID,
			Time: time.Now().Unix(),
		},
		Message: message,
	}

	encoded, err := objects.Encode(commit)
	if err != nil {
		return "", err
	}

	return s.Put(encoded)
}
