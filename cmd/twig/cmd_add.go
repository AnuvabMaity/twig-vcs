package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/repo"
)

// runAdd implements the 'twig add' subcommand.
func runAdd() {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: twig add <path> [<path>...]")
		os.Exit(1)
	}

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

	for _, path := range args {
		if err := r.AddFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error adding file %s: %v\n", path, err)
			os.Exit(1)
		}
	}
}
