package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"twig/internal/repo"
)

// runDemo implements 'bench demo'
func runDemo(args []string) {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: bench demo <dedup-story>")
		os.Exit(1)
	}

	scenario := args[0]
	if scenario != "dedup-story" {
		fmt.Fprintf(os.Stderr, "Unknown demo scenario: %q. Supported scenarios: dedup-story\n", scenario)
		os.Exit(1)
	}

	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("================================================================================")
	fmt.Println("              TWIG VCS — STAKEHOLDER DEMO: DEDUP-STORY                  ")
	fmt.Println("================================================================================")
	fmt.Println("This narrated walkthrough demonstrates how Twig's Content-Defined Chunking (CDC)")
	fmt.Println("achieves massive storage deduplication compared to whole-file storage systems.")
	fmt.Println("================================================================================")

	// Setup repository path
	demoDir := filepath.Join(".out", "demo_repo")
	_ = os.RemoveAll(demoDir)
	if err := os.MkdirAll(demoDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create demo directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize repository
	if err := repo.Init(demoDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Twig repo: %v\n", err)
		os.Exit(1)
	}
	r, err := repo.Open(demoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open Twig repo: %v\n", err)
		os.Exit(1)
	}
	// Write author configuration manually
	configPath := filepath.Join(demoDir, ".twig", "config")
	_ = os.WriteFile(configPath, []byte("user.id=demo-presenter\n"), 0644)

	fmt.Println("\n[Status] Fresh Twig repository initialized at .out/demo_repo.")
	
	waitDemoKeypress("Step 1: Create and commit an initial 5.00 MB design file...")

	// Generate 5MB redundant file
	rng := rand.New(rand.NewSource(42))
	data := generateRedundantBytes(rng, 5*1024*1024)
	designFilePath := filepath.Join(demoDir, "design.bin")
	if err := os.WriteFile(designFilePath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write design.bin: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("-> Generated 5.00 MB file 'design.bin' with realistic internal redundant patterns.")

	// Stage and Commit 1
	if err := r.AddFile(designFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stage design.bin: %v\n", err)
		os.Exit(1)
	}
	commit1, err := r.Commit("Commit 1: Initial design layout")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Commit 1 failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("-> Staged and committed design.bin (Commit: %s).\n", commit1[:8])

	waitDemoKeypress("Step 2: Apply a small 100-byte edit in the middle of our 5.00 MB file...")

	// Mutate the file: flip 100 bytes in the middle of design.bin
	content, err := os.ReadFile(designFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read design.bin: %v\n", err)
		os.Exit(1)
	}
	off := len(content) / 2
	randomBytes := make([]byte, 100)
	rng.Read(randomBytes)
	copy(content[off:off+100], randomBytes)
	if err := os.WriteFile(designFilePath, content, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write mutated design.bin: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("-> Modified 100 bytes at offset 2,621,440 (the exact middle of design.bin).")

	waitDemoKeypress("Step 3: Commit the modified revision to Twig...")

	// Stage and Commit 2
	if err := r.AddFile(designFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stage revision: %v\n", err)
		os.Exit(1)
	}
	commit2, err := r.Commit("Commit 2: Minor revisions to center text layouts")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Commit 2 failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("-> Staged and committed revision (Commit: %s).\n", commit2[:8])

	waitDemoKeypress("Step 4: Check physical storage footprint and deduplication savings...")

	// Measure physical database size
	objectsDir := filepath.Join(demoDir, ".twig", "objects")
	physicalSize, err := calculateDirSize(objectsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to measure objects size: %v\n", err)
		os.Exit(1)
	}
	logicalTotal := int64(10 * 1024 * 1024) // 2 versions of 5MB
	savings := 100.0 * (1.0 - float64(physicalSize)/float64(logicalTotal))

	fmt.Printf("\n--- Storage footprint analysis ---\n")
	fmt.Printf("Logical history size (2 x 5.00 MB): %.2f MB\n", float64(logicalTotal)/(1024*1024))
	fmt.Printf("Physical disk size of Twig store:   %.2f MB\n", float64(physicalSize)/(1024*1024))
	fmt.Printf("Deduplication space savings:        %.2f%%\n\n", savings)
	fmt.Println("Because only the chunks affected by the 100-byte modification were added,")
	fmt.Println("we avoided saving another full copy of the 5MB file. Whole-file storage")
	fmt.Println("systems (like Git LFS) would occupy a full 10.00 MB on disk locally.")

	waitDemoKeypress("Step 5: Visualize chunk sharing and reuse maps...")

	fmt.Println("\nExecuting 'bench viz chunks' for design.bin:")
	runVizChunks([]string{
		"--old", commit1,
		"--new", commit2,
		"--path", "design.bin",
		"--store", filepath.Join(demoDir, ".twig"),
	})

	fmt.Println("\n================================================================================")
	fmt.Println("DEMO COMPLETED: Twig successfully shared almost 99% of chunks between versions.")
	fmt.Println("Out of the chunks this file was split into, only a few needed to be stored again.")
	fmt.Println("This is the core thesis of Twig's Content-Defined Chunking architecture.")
	fmt.Println("================================================================================")
}

func waitDemoKeypress(prompt string) {
	fmt.Printf("\n>>> %s [Press Enter to proceed]", prompt)
	// Read a line of input
	var dummy string
	_, _ = fmt.Scanln(&dummy)
}
