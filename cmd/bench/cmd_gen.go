package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"twig/internal/repo"
)

// runGenCorpus implements 'bench gen corpus'
func runGenCorpus(args []string) {
	fs := flag.NewFlagSet("gen corpus", flag.ExitOnError)
	outDir := fs.String("out", "", "Directory to write the corpus files (required)")
	profile := fs.String("profile", "mixed", "Size profile: small, mixed, large")
	count := fs.Int("count", 10, "Number of files to generate")
	seed := fs.Int64("seed", 42, "Random seed for reproducibility")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --out directory is required")
		os.Exit(1)
	}

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	rng := rand.New(rand.NewSource(*seed))

	fmt.Printf("Generating corpus of %d files in %q (profile: %s, seed: %d)...\n", *count, *outDir, *profile, *seed)
	for i := 0; i < *count; i++ {
		var targetSize int
		switch *profile {
		case "small":
			// 100B to 63KB (mostly Blobs)
			targetSize = rng.Intn(63*1024-100) + 100
		case "large":
			// 64KB to 1MB (mostly Assets)
			targetSize = rng.Intn(1024*1024-64*1024) + 64*1024
		case "mixed":
			// 60% small, 40% large
			if rng.Float64() < 0.6 {
				targetSize = rng.Intn(63*1024-100) + 100
			} else {
				targetSize = rng.Intn(1024*1024-64*1024) + 64*1024
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown profile: %s\n", *profile)
			os.Exit(1)
		}

		data := generateRedundantBytes(rng, targetSize)
		filename := filepath.Join(*outDir, fmt.Sprintf("file_%d.bin", i))
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file %s: %v\n", filename, err)
			os.Exit(1)
		}
	}
	fmt.Println("Corpus generation completed successfully.")
}

// runGenRepo implements 'bench gen repo'
func runGenRepo(args []string) {
	fs := flag.NewFlagSet("gen repo", flag.ExitOnError)
	outDir := fs.String("out", "", "Directory to initialize the repo (required)")
	commits := fs.Int("commits", 5, "Number of commits to generate")
	filesPerCommit := fs.Int("files-per-commit", 3, "Number of new files to generate per commit")
	profile := fs.String("size-profile", "mixed", "Size profile of corpus files: small, mixed, large")
	seed := fs.Int64("seed", 42, "Random seed")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --out directory is required")
		os.Exit(1)
	}

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// 1. Initialize repo
	if err := repo.Init(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize repository at %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	r, err := repo.Open(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open repository: %v\n", err)
		os.Exit(1)
	}

	// Setup author config
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte("user.id=bench-generator\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write author config: %v\n", err)
		os.Exit(1)
	}

	rng := rand.New(rand.NewSource(*seed))
	var trackedFiles []string

	fmt.Printf("Generating mock repository at %s with %d commits...\n", *outDir, *commits)

	for c := 0; c < *commits; c++ {
		// Decide operations:
		// First commit: create initial files
		// Subsequent commits: 70% chance to mutate existing files, 30% to add new ones
		if c == 0 || len(trackedFiles) == 0 {
			// Generate initial files
			for i := 0; i < *filesPerCommit; i++ {
				filename := fmt.Sprintf("file_%d_%d.bin", c, i)
				filePath := filepath.Join(*outDir, filename)
				sz := getProfileSize(rng, *profile)
				data := generateRedundantBytes(rng, sz)
				if err := os.WriteFile(filePath, data, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write file %s: %v\n", filename, err)
					os.Exit(1)
				}
				trackedFiles = append(trackedFiles, filePath)
				if err := r.AddFile(filePath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to stage %s: %v\n", filePath, err)
					os.Exit(1)
				}
			}
		} else {
			if rng.Float64() < 0.7 {
				// Mutate 1 to 3 existing files
				numMutations := rng.Intn(minInt(len(trackedFiles), 3)) + 1
				for m := 0; m < numMutations; m++ {
					targetIdx := rng.Intn(len(trackedFiles))
					filePath := trackedFiles[targetIdx]
					mutateFileBytes(rng, filePath)
					if err := r.AddFile(filePath); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to stage mutated file %s: %v\n", filePath, err)
						os.Exit(1)
					}
				}
			} else {
				// Add a new file
				filename := fmt.Sprintf("file_%d_new.bin", c)
				filePath := filepath.Join(*outDir, filename)
				sz := getProfileSize(rng, *profile)
				data := generateRedundantBytes(rng, sz)
				if err := os.WriteFile(filePath, data, 0644); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to write file %s: %v\n", filename, err)
					os.Exit(1)
				}
				trackedFiles = append(trackedFiles, filePath)
				if err := r.AddFile(filePath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to stage %s: %v\n", filePath, err)
					os.Exit(1)
				}
			}
		}

		commitMsg := fmt.Sprintf("bench: commit %d", c+1)
		hash, err := r.Commit(commitMsg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Commit failed at iteration %d: %v\n", c+1, err)
			os.Exit(1)
		}
		fmt.Printf("  Commit %d: %s %q\n", c+1, hash[:10], commitMsg)
	}

	fmt.Println("Repository generation completed successfully.")
}

// runGenConflictScenario implements 'bench gen conflict-scenario'
func runGenConflictScenario(args []string) {
	fs := flag.NewFlagSet("gen conflict-scenario", flag.ExitOnError)
	outDir := fs.String("out", "", "Directory to write the conflict repo (required)")
	kind := fs.String("kind", "conflicting", "Conflict kind: clean, conflicting, delete-vs-modify")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --out directory is required")
		os.Exit(1)
	}

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	if err := repo.Init(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init repo at %s: %v\n", *outDir, err)
		os.Exit(1)
	}

	r, err := repo.Open(*outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open repo: %v\n", err)
		os.Exit(1)
	}

	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte("user.id=conflict-scenario\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write author config: %v\n", err)
		os.Exit(1)
	}

	// 1. Initial Commit
	file1Path := filepath.Join(*outDir, "file1.txt")
	if err := os.WriteFile(file1Path, []byte("initial content for file1\n"), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write initial file1.txt: %v\n", err)
		os.Exit(1)
	}
	if err := r.AddFile(file1Path); err != nil {
		fmt.Fprintf(os.Stderr, "Add file1 failed: %v\n", err)
		os.Exit(1)
	}

	if _, err := r.Commit("Initial commit on main"); err != nil {
		fmt.Fprintf(os.Stderr, "Initial commit failed: %v\n", err)
		os.Exit(1)
	}

	// 2. Create feature branch
	if err := r.CreateBranch("feature"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create branch: %v\n", err)
		os.Exit(1)
	}

	switch *kind {
	case "clean":
		// Checkout feature, add file2.txt
		if err := r.Checkout("feature", true); err != nil {
			fmt.Fprintf(os.Stderr, "Checkout feature branch failed: %v\n", err)
			os.Exit(1)
		}
		file2Path := filepath.Join(*outDir, "file2.txt")
		if err := os.WriteFile(file2Path, []byte("feature branch file2 content\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write file2.txt: %v\n", err)
			os.Exit(1)
		}
		if err := r.AddFile(file2Path); err != nil {
			fmt.Fprintf(os.Stderr, "Add file2 failed: %v\n", err)
			os.Exit(1)
		}
		if _, err := r.Commit("Commit on feature branch"); err != nil {
			fmt.Fprintf(os.Stderr, "Commit on feature failed: %v\n", err)
			os.Exit(1)
		}

		// Checkout main, add file3.txt
		if err := r.Checkout("main", true); err != nil {
			fmt.Fprintf(os.Stderr, "Checkout main branch failed: %v\n", err)
			os.Exit(1)
		}
		file3Path := filepath.Join(*outDir, "file3.txt")
		if err := os.WriteFile(file3Path, []byte("main branch file3 content\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write file3.txt: %v\n", err)
			os.Exit(1)
		}
		if err := r.AddFile(file3Path); err != nil {
			fmt.Fprintf(os.Stderr, "Add file3 failed: %v\n", err)
			os.Exit(1)
		}
		if _, err := r.Commit("Commit on main branch"); err != nil {
			fmt.Fprintf(os.Stderr, "Commit on main failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Successfully generated clean non-overlapping branches scenario.")
		fmt.Printf("To merge feature into main run: cd %s && twig merge feature\n", *outDir)

	case "conflicting":
		// Checkout feature, modify file1.txt differently
		if err := r.Checkout("feature", true); err != nil {
			fmt.Fprintf(os.Stderr, "Checkout feature branch failed: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(file1Path, []byte("feature edited content!\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write file1.txt: %v\n", err)
			os.Exit(1)
		}
		if err := r.AddFile(file1Path); err != nil {
			fmt.Fprintf(os.Stderr, "Add file1 failed: %v\n", err)
			os.Exit(1)
		}
		if _, err := r.Commit("Feature branch modification"); err != nil {
			fmt.Fprintf(os.Stderr, "Commit on feature failed: %v\n", err)
			os.Exit(1)
		}

		// Checkout main, modify file1.txt differently
		if err := r.Checkout("main", true); err != nil {
			fmt.Fprintf(os.Stderr, "Checkout main branch failed: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(file1Path, []byte("main branch conflicting content!\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write file1.txt: %v\n", err)
			os.Exit(1)
		}
		if err := r.AddFile(file1Path); err != nil {
			fmt.Fprintf(os.Stderr, "Add file1 failed: %v\n", err)
			os.Exit(1)
		}
		if _, err := r.Commit("Main branch modification"); err != nil {
			fmt.Fprintf(os.Stderr, "Commit on main failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Successfully generated conflicting edits branches scenario.")
		fmt.Printf("To run conflict merge run: cd %s && twig merge feature\n", *outDir)
		fmt.Println("Resolve with: twig resolve --ours file1.txt | --theirs file1.txt")

	case "delete-vs-modify":
		// Checkout feature, modify file1.txt
		if err := r.Checkout("feature", true); err != nil {
			fmt.Fprintf(os.Stderr, "Checkout feature branch failed: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(file1Path, []byte("feature branch modified file1\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write file1.txt: %v\n", err)
			os.Exit(1)
		}
		if err := r.AddFile(file1Path); err != nil {
			fmt.Fprintf(os.Stderr, "Add file1 failed: %v\n", err)
			os.Exit(1)
		}
		if _, err := r.Commit("Feature branch modify"); err != nil {
			fmt.Fprintf(os.Stderr, "Commit on feature failed: %v\n", err)
			os.Exit(1)
		}

		// Checkout main, delete file1.txt
		if err := r.Checkout("main", true); err != nil {
			fmt.Fprintf(os.Stderr, "Checkout main branch failed: %v\n", err)
			os.Exit(1)
		}
		if err := os.Remove(file1Path); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete file1.txt: %v\n", err)
			os.Exit(1)
		}
		if err := r.AddFile(file1Path); err != nil { // Stage deletion
			fmt.Fprintf(os.Stderr, "Stage deletion of file1 failed: %v\n", err)
			os.Exit(1)
		}
		if _, err := r.Commit("Main branch deleted file1"); err != nil {
			fmt.Fprintf(os.Stderr, "Commit on main failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Successfully generated delete-vs-modify branches scenario.")
		fmt.Printf("To merge feature into main run: cd %s && twig merge feature\n", *outDir)

	default:
		fmt.Fprintf(os.Stderr, "Unknown conflict scenario kind: %s\n", *kind)
		os.Exit(1)
	}
}

// Helpers
func generateRedundantBytes(rng *rand.Rand, targetSize int) []byte {
	const blockSize = 2048
	templates := make([][]byte, 5)
	for i := range templates {
		templates[i] = make([]byte, blockSize)
		rng.Read(templates[i])
	}

	res := make([]byte, 0, targetSize)
	for len(res) < targetSize {
		remaining := targetSize - len(res)
		if rng.Float64() < 0.3 && remaining >= blockSize {
			t := templates[rng.Intn(len(templates))]
			res = append(res, t...)
		} else {
			chunkSz := rng.Intn(2048) + 512
			if chunkSz > remaining {
				chunkSz = remaining
			}
			newChunk := make([]byte, chunkSz)
			rng.Read(newChunk)
			res = append(res, newChunk...)
		}
	}
	return res
}

func getProfileSize(rng *rand.Rand, profile string) int {
	switch profile {
	case "small":
		return rng.Intn(63*1024-100) + 100
	case "large":
		return rng.Intn(1024*1024-64*1024) + 64*1024
	case "mixed":
		if rng.Float64() < 0.6 {
			return rng.Intn(63*1024-100) + 100
		}
		return rng.Intn(1024*1024-64*1024) + 64*1024
	default:
		return 10 * 1024
	}
}

func mutateFileBytes(rng *rand.Rand, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	sz := len(content)
	if sz < 100 {
		return
	}
	// Flip 50 bytes in place
	off := rng.Intn(sz - 50)
	randomBytes := make([]byte, 50)
	rng.Read(randomBytes)
	copy(content[off:off+50], randomBytes)
	_ = os.WriteFile(filePath, content, 0644)
}

func randString(rng *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
