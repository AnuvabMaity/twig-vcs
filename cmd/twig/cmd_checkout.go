package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/repo"
)

// runCheckout implements the 'twig checkout' subcommand.
func runCheckout() {
	fs := flag.NewFlagSet("checkout", flag.ExitOnError)
	forceOpt := fs.Bool("force", false, "force checkout (overwrite local changes)")

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: twig checkout [--force] <ref>")
		os.Exit(1)
	}

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: twig checkout [--force] <ref>")
		os.Exit(1)
	}

	ref := args[0]

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

	if err := r.Checkout(ref, *forceOpt); err != nil {
		fmt.Fprintf(os.Stderr, "Error checking out: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Checked out %s\n", ref)
}
