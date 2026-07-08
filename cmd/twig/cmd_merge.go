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
	abort := fs.Bool("abort", false, "Abort the current merge process and restore the pre-merge state")

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
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

	if *abort {
		err = r.AbortMerge()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error aborting merge: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Merge aborted successfully. Working copy and index restored to HEAD.")
		return
	}

	args := fs.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: twig merge [--abort] | twig merge <branch>")
		os.Exit(1)
	}

	branchName := args[0]

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
