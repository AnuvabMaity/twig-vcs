package main

import (
	"errors"
	"fmt"
	"os"
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

	headCommitHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		if errors.Is(err, refs.ErrUnbornBranch) {
			fmt.Fprintln(os.Stderr, "Error: branch has no commits yet")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error resolving HEAD: %v\n", err)
		os.Exit(1)
	}

	entries, err := r.Log(headCommitHash)
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
