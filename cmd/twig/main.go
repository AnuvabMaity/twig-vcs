package main

import (
	"fmt"
	"os"
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
	fmt.Fprintln(os.Stderr, "  hash-object  Compute object ID and optionally create a blob from a file")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
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
	case "hash-object":
		runHashObject()
	default:
		printUsage()
		os.Exit(1)
	}
}

func runInit() {
	fmt.Println("init: not implemented")
}

func runAdd() {
	fmt.Println("add: not implemented")
}

func runCommit() {
	fmt.Println("commit: not implemented")
}

func runLog() {
	fmt.Println("log: not implemented")
}

func runCheckout() {
	fmt.Println("checkout: not implemented")
}

func runStatus() {
	fmt.Println("status: not implemented")
}

func runBranch() {
	fmt.Println("branch: not implemented")
}

