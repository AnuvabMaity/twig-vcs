package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/ingest"
	"twig/internal/store"
)

// runHashObject implements the 'twig hash-object' subcommand.
func runHashObject() {
	fs := flag.NewFlagSet("hash-object", flag.ExitOnError)
	storeDir := fs.String("store", "./.twig", "path to the object store")

	// Parse flags starting from os.Args[2:]
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: twig hash-object [--store <dir>] <file>")
		os.Exit(1)
	}

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Usage: twig hash-object [--store <dir>] <file>")
		os.Exit(1)
	}

	filePath := args[0]

	s := store.Open(*storeDir)
	if err := s.EnsureLayout(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing store layout: %v\n", err)
		os.Exit(1)
	}

	hash, _, err := ingest.IngestFile(s, filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ingesting file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
