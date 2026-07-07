package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"twig/internal/repo"
)

// runResolve implements the 'twig resolve' subcommand.
func runResolve() {
	fs := flag.NewFlagSet("resolve", flag.ExitOnError)
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: twig resolve <ours|theirs> <file>")
		os.Exit(1)
	}

	side := args[0]
	filePath := args[1]

	if side != "ours" && side != "theirs" {
		fmt.Fprintln(os.Stderr, "Error: side must be either 'ours' or 'theirs'")
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

	err = r.ResolveConflict(filePath, side)
	if err != nil {
		if errors.Is(err, repo.ErrNoConflict) {
			fmt.Fprintf(os.Stderr, "Error: no conflict on path %s\n", filePath)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error resolving conflict: %v\n", err)
		os.Exit(1)
	}
}
