package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"twig/internal/refs"
	"twig/internal/repo"
)

// runBranch implements the 'twig branch' subcommand.
func runBranch() {
	fs := flag.NewFlagSet("branch", flag.ExitOnError)
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
	args := fs.Args()
	if len(args) == 0 {
		// List branches
		branches, err := refs.ListBranches(r.TwigDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing branches: %v\n", err)
			os.Exit(1)
		}
		// Sort branch names
		sort.Strings(branches)
		// Get current branch from HEAD
		currTarget, isBranch, err := refs.ReadHEAD(r.TwigDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading HEAD: %v\n", err)
			os.Exit(1)
		}
		for _, b := range branches {
			if isBranch && b == currTarget {
				fmt.Printf("* %s\n", b)
			} else {
				fmt.Printf("  %s\n", b)
			}
		}
	} else if len(args) == 1 {
		// Create branch
		branchName := args[0]
		err := r.CreateBranch(branchName)
		if err != nil {
			if errors.Is(err, repo.ErrBranchExists) {
				fmt.Fprintf(os.Stderr, "Error: branch %s already exists\n", branchName)
				os.Exit(1)
			}
			if errors.Is(err, refs.ErrUnbornBranch) {
				fmt.Fprintln(os.Stderr, "Error: cannot create branch on an unborn branch")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Error creating branch: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Usage: twig branch [<branch-name>]")
		os.Exit(1)
	}
}
