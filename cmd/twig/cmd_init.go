package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/repo"
)

// runInit implements the 'twig init' subcommand.
func runInit() {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	targetDir := "."
	args := fs.Args()
	if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "Usage: twig init [<directory>]")
		os.Exit(1)
	}
	if len(args) == 1 {
		targetDir = args[0]
	}

	if err := repo.Init(targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing repository: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initialized empty Twig repository in %s/.twig/\n", targetDir)
}
