package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"twig/internal/refs"
	"twig/internal/repo"
)

// runLog implements the 'twig log' subcommand.
func runLog() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
		os.Exit(1)
	}

	r, err := repo.Open(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var startCommit string
	if len(os.Args) >= 3 {
		targetRef := os.Args[2]
		resolved, err := resolveLogRef(r.TwigDir, targetRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving reference %q: %v\n", targetRef, err)
			os.Exit(1)
		}
		startCommit = resolved
	} else {
		headCommitHash, err := refs.ResolveHEAD(r.TwigDir)
		if err != nil {
			if errors.Is(err, refs.ErrUnbornBranch) {
				fmt.Fprintln(os.Stderr, "Error: branch has no commits yet")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Error resolving HEAD: %v\n", err)
			os.Exit(1)
		}
		startCommit = headCommitHash
	}

	entries, err := r.Log(startCommit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting log: %v\n", err)
		os.Exit(1)
	}

	for i, entry := range entries {
		if i > 0 {
			fmt.Println()
		}
		// Print short hash (7 characters)
		shortHash := entry.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		fmt.Printf("commit %s\n", shortHash)
		fmt.Printf("Author: %s\n", entry.Commit.Author.ID)
		fmt.Printf("Date: %d\n\n", entry.Commit.Author.Time)

		message := strings.TrimRight(entry.Commit.Message, "\r\n")
		lines := strings.Split(message, "\n")
		for _, line := range lines {
			fmt.Printf("    %s\n", line)
		}
	}
}

func resolveLogRef(twigDir string, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("ref cannot be empty")
	}
	if commitHash, err := refs.ReadBranch(twigDir, input); err == nil {
		return commitHash, nil
	}
	if input == "HEAD" {
		return refs.ResolveHEAD(twigDir)
	}
	isHex := true
	for _, c := range input {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			isHex = false
			break
		}
	}
	if isHex {
		lowerInput := strings.ToLower(input)
		if len(lowerInput) == 64 {
			return lowerInput, nil
		}
		if len(lowerInput) >= 7 {
			objectsDir := filepath.Join(twigDir, "objects")
			prefixDir := lowerInput[:2]
			restPrefix := lowerInput[2:]
			searchDir := filepath.Join(objectsDir, prefixDir)
			files, err := os.ReadDir(searchDir)
			if err == nil {
				var matches []string
				for _, f := range files {
					if !f.IsDir() && strings.HasPrefix(strings.ToLower(f.Name()), restPrefix) {
						matches = append(matches, prefixDir+f.Name())
					}
				}
				if len(matches) == 1 {
					return matches[0], nil
				}
				if len(matches) > 1 {
					return "", fmt.Errorf("short hash %q is ambiguous", input)
				}
			}
		}
	}
	return "", fmt.Errorf("reference not found")
}
