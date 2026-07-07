package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/repo"
)

// runCommit implements the 'twig commit' subcommand.
func runCommit() {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	msgOpt := fs.String("m", "", "commit message")

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: twig commit -m \"<message>\"")
		os.Exit(1)
	}

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *msgOpt == "" {
		fmt.Fprintln(os.Stderr, "Error: switch 'm' requires a value")
		fmt.Fprintln(os.Stderr, "Usage: twig commit -m \"<message>\"")
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

	commitHash, err := r.Commit(*msgOpt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Commit failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[%s] %s\n", commitHash[:7], *msgOpt)
}
