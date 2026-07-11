package main

import (
	"encoding/json"
	"fmt"
	"os"

	"twig/internal/metrics"
)

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: twig <command> [<args>]")
	fmt.Fprintln(os.Stderr, "Available commands:")
	fmt.Fprintln(os.Stderr, "  init         Initialize a new, empty repository")
	fmt.Fprintln(os.Stderr, "  add          Add file contents to the staging area")
	fmt.Fprintln(os.Stderr, "  commit       Record changes to the repository")
	fmt.Fprintln(os.Stderr, "  log          Show commit history")
	fmt.Fprintln(os.Stderr, "  checkout     Switch branches or restore working tree files")
	fmt.Fprintln(os.Stderr, "  status       Show the working tree status")
	fmt.Fprintln(os.Stderr, "  branch       List, create, or delete branches")
	fmt.Fprintln(os.Stderr, "  merge        Join two or more development histories together")
	fmt.Fprintln(os.Stderr, "  resolve      Resolve merge conflicts in staging index and working copy")
}

func main() {
	if os.Getenv("TWIG_METRICS") == "1" {
		defer func() {
			snapshot := metrics.Snapshot()
			if data, err := json.Marshal(snapshot); err == nil {
				fmt.Fprintf(os.Stderr, "\nTWIG_METRICS_DUMP:%s\n", string(data))
			}
		}()
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd == "--help" || cmd == "-h" || cmd == "help" {
		printUsage()
		os.Exit(0)
	}

	switch cmd {
	case "init":
		runInit()
	case "add":
		runAdd()
	case "commit":
		runCommit()
	case "log":
		runLog()
	case "checkout":
		runCheckout()
	case "status":
		runStatus()
	case "branch":
		runBranch()
	case "merge":
		runMerge()
	case "resolve":
		runResolve()
	case "hash-object":
		runHashObject()
	case "cat-object":
		runCatObject()
	default:
		printUsage()
		os.Exit(1)
	}
}
