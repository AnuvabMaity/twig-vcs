package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"twig/internal/repo"
)

// runMerge implements the 'twig merge' subcommand.
func runMerge() {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: twig merge <branch>")
		os.Exit(1)
	}

	branchName := args[0]

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

	err = r.Merge(branchName)
	if err != nil {
		if errors.Is(err, repo.ErrMergeConflicts) {
			fmt.Fprintln(os.Stderr, "Conflicts:")
			var mergeErr *repo.MergeConflictsError
			if errors.As(err, &mergeErr) {
				for _, c := range mergeErr.Conflicts {
					fmt.Fprintf(os.Stderr, "  %s\n", c)
				}
			}
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error merging: %v\n", err)
		os.Exit(1)
	}
}
