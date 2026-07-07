package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/repo"
)

// runStatus implements the 'twig status' subcommand.
func runStatus() {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if len(fs.Args()) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: twig status")
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
		os.Exit(1)
	}

	r, err := repo.Open(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening repository: %v\n", err)
		os.Exit(1)
	}

	entries, err := r.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting status: %v\n", err)
		os.Exit(1)
	}

	var staged []string
	var unmerged []string
	var notStaged []string
	var untracked []string

	for _, entry := range entries {
		switch entry.Status {
		case repo.StatusStagedNew:
			staged = append(staged, fmt.Sprintf("\tnew file:   %s", entry.Path))
		case repo.StatusStagedModified:
			staged = append(staged, fmt.Sprintf("\tmodified:   %s", entry.Path))
		case repo.StatusConflict:
			unmerged = append(unmerged, fmt.Sprintf("\tboth modified:      %s", entry.Path))
		case repo.StatusModified:
			notStaged = append(notStaged, fmt.Sprintf("\tmodified:   %s", entry.Path))
		case repo.StatusDeleted:
			notStaged = append(notStaged, fmt.Sprintf("\tdeleted:    %s", entry.Path))
		case repo.StatusUntracked:
			untracked = append(untracked, fmt.Sprintf("\t%s", entry.Path))
		}
	}

	hasOutput := false

	if len(unmerged) > 0 {
		fmt.Println("Unmerged paths:")
		fmt.Println("  (use \"twig resolve <--ours|--theirs> <file>\" to resolve conflicts)")
		fmt.Println()
		for _, line := range unmerged {
			fmt.Println(line)
		}
		fmt.Println()
		hasOutput = true
	}

	if len(staged) > 0 {
		fmt.Println("Changes to be committed:")
		for _, line := range staged {
			fmt.Println(line)
		}
		fmt.Println()
		hasOutput = true
	}

	if len(notStaged) > 0 {
		fmt.Println("Changes not staged for commit:")
		fmt.Println("  (use \"twig add <file>...\" to update what will be committed)")
		fmt.Println()
		for _, line := range notStaged {
			fmt.Println(line)
		}
		fmt.Println()
		hasOutput = true
	}

	if len(untracked) > 0 {
		fmt.Println("Untracked files:")
		fmt.Println("  (use \"twig add <file>...\" to include in what will be committed)")
		fmt.Println()
		for _, line := range untracked {
			fmt.Println(line)
		}
		fmt.Println()
		hasOutput = true
	}

	if !hasOutput {
		fmt.Println("nothing to commit, working tree clean")
	}
}
