package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/objects"
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
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	// For Phase 1, we always wrap the content as a Blob
	blob := objects.Blob{
		Type: objects.TypeBlob,
		Data: data,
	}

	encoded, err := objects.Encode(blob)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding blob: %v\n", err)
		os.Exit(1)
	}

	s := store.Open(*storeDir)
	if err := s.EnsureLayout(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing store layout: %v\n", err)
		os.Exit(1)
	}

	hash, err := s.Put(encoded)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error storing object: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
