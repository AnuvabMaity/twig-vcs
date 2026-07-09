package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"twig/internal/objects"
	"twig/internal/refs"
	"twig/internal/repo"
)

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: bench <command> [<args>]")
	fmt.Fprintln(os.Stderr, "Available commands:")
	fmt.Fprintln(os.Stderr, "  cat index              Display the staging index contents")
	fmt.Fprintln(os.Stderr, "  cat object             Display a decompressed CBOR object or raw chunk")
	fmt.Fprintln(os.Stderr, "  check integrity        Validate repository structural consistency and object hashes")
	fmt.Fprintln(os.Stderr, "  viz chunks             Display a visual map of chunk sharing between versions")
	fmt.Fprintln(os.Stderr, "  viz store-stats        Display reachable vs orphaned storage counts")
	fmt.Fprintln(os.Stderr, "  viz commit-graph       Render the commit history dependency graph")
	fmt.Fprintln(os.Stderr, "  viz tree               Display a hierarchical file and object chunk tree")
	fmt.Fprintln(os.Stderr, "  time                   Time and monitor Twig command runs with internal counters")
	fmt.Fprintln(os.Stderr, "  gen corpus             Generate synthetic files with realistic redundancy")
	fmt.Fprintln(os.Stderr, "  gen repo               Spin up a fully populated mock repository")
	fmt.Fprintln(os.Stderr, "  gen conflict-scenario  Generate a repository with branch conflict scenarios")
	fmt.Fprintln(os.Stderr, "  gen dataset            Curate SQLite/Assets/JPEGs benchmark datasets")
	fmt.Fprintln(os.Stderr, "  mutate                 Apply controlled byte mutations to a file")
	fmt.Fprintln(os.Stderr, "  run-benchmark          Run execution speed and database size benchmarks")
	fmt.Fprintln(os.Stderr, "  demo <scenario>        Run a narrated stakeholder presentation walkthrough")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "cat":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: bench cat <index|object> [<args>]")
			os.Exit(1)
		}
		sub := os.Args[2]
		switch sub {
		case "index":
			runCatIndex(os.Args[3:])
		case "object":
			runCatObject(os.Args[3:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown subcommand: bench cat %s\n", sub)
			os.Exit(1)
		}
	case "check":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: bench check <integrity> [<args>]")
			os.Exit(1)
		}
		sub := os.Args[2]
		switch sub {
		case "integrity":
			runCheckIntegrity(os.Args[3:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown subcommand: bench check %s\n", sub)
			os.Exit(1)
		}
	case "viz":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: bench viz <chunks|store-stats|commit-graph|tree> [<args>]")
			os.Exit(1)
		}
		sub := os.Args[2]
		switch sub {
		case "chunks":
			runVizChunks(os.Args[3:])
		case "store-stats":
			runVizStoreStats(os.Args[3:])
		case "commit-graph":
			runVizCommitGraph(os.Args[3:])
		case "tree":
			runVizTree(os.Args[3:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown subcommand: bench viz %s\n", sub)
			os.Exit(1)
		}
	case "gen":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: bench gen <corpus|repo|conflict-scenario|dataset> [<args>]")
			os.Exit(1)
		}
		sub := os.Args[2]
		switch sub {
		case "corpus":
			runGenCorpus(os.Args[3:])
		case "repo":
			runGenRepo(os.Args[3:])
		case "conflict-scenario":
			runGenConflictScenario(os.Args[3:])
		case "dataset":
			runGenDataset(os.Args[3:])
		default:
			fmt.Fprintf(os.Stderr, "Unknown subcommand: bench gen %s\n", sub)
			os.Exit(1)
		}
	case "mutate":
		runMutate(os.Args[2:])
	case "run-benchmark":
		runBenchmark(os.Args[2:])
	case "demo":
		runDemo(os.Args[2:])
	case "time":
		runBenchTime(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

// FindTwigDir searches upward from the current directory for the .twig directory.
func FindTwigDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	_, twigDir, err := repo.FindRoot(cwd)
	if err != nil {
		return "", err
	}
	return twigDir, nil
}

// ResolveRefOrHash resolves a branch ref, HEAD, full hash, or short hash prefix
// to a full 64-character BLAKE3 hex-encoded hash.
func ResolveRefOrHash(twigDir string, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("ref or hash input cannot be empty")
	}

	// 1. Check if it is a branch name
	if commitHash, err := refs.ReadBranch(twigDir, input); err == nil {
		return commitHash, nil
	}

	// 2. Check if it is HEAD
	if input == "HEAD" {
		if commitHash, err := refs.ResolveHEAD(twigDir); err == nil {
			return commitHash, nil
		}
	}

	// 3. Check if it is a hex hash (short or full)
	isHex := true
	for _, c := range input {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			isHex = false
			break
		}
	}

	if isHex {
		lowerInput := strings.ToLower(input)
		if len(lowerInput) == 64 {
			return lowerInput, nil
		}
		if len(lowerInput) >= 7 {
			// Find object by short prefix
			objectsDir := filepath.Join(twigDir, objects.ObjectsDirName)
			prefixDir := lowerInput[:2]
			restPrefix := lowerInput[2:]

			searchDir := filepath.Join(objectsDir, prefixDir)
			files, err := os.ReadDir(searchDir)
			if err != nil {
				if os.IsNotExist(err) {
					return "", fmt.Errorf("object hash prefix %q not found in store", input)
				}
				return "", fmt.Errorf("failed to search object directory: %w", err)
			}

			var matches []string
			for _, f := range files {
				if !f.IsDir() && strings.HasPrefix(strings.ToLower(f.Name()), restPrefix) {
					matches = append(matches, prefixDir+f.Name())
				}
			}

			if len(matches) == 0 {
				return "", fmt.Errorf("object hash prefix %q not found in store", input)
			}
			if len(matches) > 1 {
				return "", fmt.Errorf("short hash prefix %q is ambiguous (matches %d objects: %v)", input, len(matches), matches)
			}
			return matches[0], nil
		}
		return "", fmt.Errorf("short hash must be at least 7 characters")
	}

	return "", fmt.Errorf("failed to resolve %q as a branch, HEAD, or hash", input)
}
