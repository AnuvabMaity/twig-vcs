package main

import (
	"flag"
	"fmt"
	"os"

	"twig/internal/ingest"
	"twig/internal/objects"
	"twig/internal/store"
)

// runCatObject implements the 'twig cat-object' subcommand.
func runCatObject() {
	fs := flag.NewFlagSet("cat-object", flag.ExitOnError)
	storeDir := fs.String("store", "./.twig", "path to the object store")

	// Parse flags starting from os.Args[2:]
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: twig cat-object [--store <dir>] <hash> <type>")
		os.Exit(1)
	}

	if err := fs.Parse(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	args := fs.Args()
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: twig cat-object [--store <dir>] <hash> <type>")
		os.Exit(1)
	}

	hash := args[0]
	typeStr := args[1]

	var objType objects.ObjectType
	switch typeStr {
	case "blob":
		objType = objects.TypeBlob
	case "asset":
		objType = objects.TypeAsset
	default:
		fmt.Fprintf(os.Stderr, "Error: unrecognized type %q. Allowed values: blob, asset\n", typeStr)
		os.Exit(1)
	}

	s := store.Open(*storeDir)
	if err := s.EnsureLayout(); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening store layout: %v\n", err)
		os.Exit(1)
	}

	if err := ingest.Reconstruct(s, hash, objType, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "Error reconstructing object: %v\n", err)
		os.Exit(1)
	}
}
