package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type BenchmarkResult struct {
	Revision   int
	Duration   time.Duration
	FolderSize int64
}

// runBenchmark implements 'bench run-benchmark'
func runBenchmark(args []string) {
	fs := flag.NewFlagSet("run-benchmark", flag.ExitOnError)
	dataset := fs.String("dataset", "", "Dataset to benchmark: sqlite, assets, jpegs (required unless --full is set)")
	datasetDir := fs.String("dataset-dir", "", "Path to the dataset directory containing revisions/ (required unless --full is set)")
	outDir := fs.String("out-dir", ".out/bench/results/raw", "Directory to write results CSVs")
	revisions := fs.Int("revisions", 20, "Number of revisions to run")
	twigPathOpt := fs.String("twig-path", "", "Path to twig binary (optional, auto-discovered if empty)")
	gitPathOpt := fs.String("git-path", "git", "Path to git binary")
	full := fs.Bool("full", false, "Run the entire dataset generation, benchmark comparison, and aggregation pipeline")
	seed := fs.Int64("seed", 42, "Seed used for dataset generation if running with --full")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// 1. Resolve VCS binary paths
	twigBinary, err := findTwigBinary(*twigPathOpt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve twig path: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Resolved twig binary path: %q\n", twigBinary)

	gitBinary := *gitPathOpt
	if err := checkGitVersion(gitBinary); err != nil {
		fmt.Fprintf(os.Stderr, "Git verify failed (is Git installed on PATH?): %v\n", err)
		os.Exit(1)
	}

	// 2. Setup output folder
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output directory: %v\n", err)
		os.Exit(1)
	}

	if *full {
		fmt.Println("Starting full automated benchmark pipeline...")
		datasets := []string{"sqlite", "assets", "jpegs"}
		for _, ds := range datasets {
			dsDir := filepath.Join(".out", "datasets", ds)
			_ = os.RemoveAll(dsDir)
			if err := generateDatasetInternal(ds, dsDir, *revisions, *seed); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to generate dataset %s: %v\n", ds, err)
				os.Exit(1)
			}

			// Run benchmarks for this dataset
			if err := runBenchmarkForDataset(ds, dsDir, *outDir, *revisions, gitBinary, twigBinary); err != nil {
				fmt.Fprintf(os.Stderr, "Benchmark failed for dataset %s: %v\n", ds, err)
				os.Exit(1)
			}
		}

		// Run Aggregation
		summaryFile := filepath.Join(".out", "bench", "results", "summary.md")
		fmt.Println("Generating aggregation summary table...")
		if err := aggregateResults(*outDir, summaryFile); err != nil {
			fmt.Fprintf(os.Stderr, "Aggregation failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Full pipeline execution completed successfully. Aggregated report saved to %s\n", summaryFile)
		return
	}

	// Single dataset run
	if *dataset == "" || *datasetDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --dataset and --dataset-dir are required unless --full is set")
		os.Exit(1)
	}

	if err := runBenchmarkForDataset(*dataset, *datasetDir, *outDir, *revisions, gitBinary, twigBinary); err != nil {
		fmt.Fprintf(os.Stderr, "Benchmark run failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\nBenchmark run completed successfully.")
}

// runBenchmarkForDataset runs the benchmark process for a single system configuration.
func runBenchmarkForDataset(dataset, datasetDir, outDir string, revisions int, gitBinary, twigBinary string) error {
	systems := []string{"git", "lfs", "twig"}
	for _, sys := range systems {
		fmt.Printf("\n==================================================\n")
		fmt.Printf("Starting Benchmark for Dataset: %s, System: %s\n", dataset, sys)
		fmt.Printf("==================================================\n")

		// Create isolated repo temporary directory
		repoDir := filepath.Join(".out", fmt.Sprintf("bench_temp_%s_%s", sys, dataset))
		_ = os.RemoveAll(repoDir)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			return fmt.Errorf("failed to create repository folder %s: %w", repoDir, err)
		}

		// Initialize repository
		if err := initSystemRepo(sys, repoDir, gitBinary, twigBinary, dataset); err != nil {
			_ = os.RemoveAll(repoDir)
			return fmt.Errorf("failed to initialize %s repository: %w", sys, err)
		}

		var results []BenchmarkResult

		// Iteration loop
		for rev := 1; rev <= revisions; rev++ {
			snapDir := filepath.Join(datasetDir, "revisions", fmt.Sprintf("rev-%02d", rev))
			if _, err := os.Stat(snapDir); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: Snapshot directory %s not found. Stopping iterations.\n", snapDir)
				break
			}

			// Clean previous files in working dir (keeping .git / .twig / .gitattributes)
			if err := cleanRepoWorkingDir(repoDir); err != nil {
				_ = os.RemoveAll(repoDir)
				return fmt.Errorf("failed to clean working directory: %w", err)
			}

			// Copy snapshot files to repo dir
			if err := copyDirectoryContent(snapDir, repoDir); err != nil {
				_ = os.RemoveAll(repoDir)
				return fmt.Errorf("failed to copy snapshot: %w", err)
			}

			// Add and Commit
			duration, err := runAddAndCommit(sys, repoDir, gitBinary, twigBinary, rev)
			if err != nil {
				_ = os.RemoveAll(repoDir)
				return fmt.Errorf("failed to run stage and commit on rev %d: %w", rev, err)
			}

			// Measure folder size of metadata directory (.git / .twig)
			metaDirName := ".git"
			if sys == "twig" {
				metaDirName = ".twig"
			}
			metaPath := filepath.Join(repoDir, metaDirName)
			size, err := calculateDirSize(metaPath)
			if err != nil {
				_ = os.RemoveAll(repoDir)
				return fmt.Errorf("failed to calculate size for %s: %w", metaPath, err)
			}

			results = append(results, BenchmarkResult{
				Revision:   rev,
				Duration:   duration,
				FolderSize: size,
			})

			fmt.Printf("  Rev %02d: Stage+Commit = %.3fs, Size = %.2f MB\n", rev, duration.Seconds(), float64(size)/(1024*1024))
		}

		// Checkout Timing Measurement
		fmt.Println("Measuring checkout performance...")
		checkoutDuration, err := runCheckoutBenchmark(sys, repoDir, gitBinary, twigBinary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Checkout benchmark failed: %v\n", err)
		} else {
			fmt.Printf("  Checkout Duration: %.3fs\n", checkoutDuration.Seconds())
		}

		// Write results to CSV
		csvPath := filepath.Join(outDir, fmt.Sprintf("%s-%s.csv", dataset, sys))
		if err := writeBenchmarkCSV(csvPath, results, checkoutDuration); err != nil {
			_ = os.RemoveAll(repoDir)
			return fmt.Errorf("failed to write CSV results: %w", err)
		}
		fmt.Printf("Benchmark CSV saved to: %s\n", csvPath)

		// Cleanup
		_ = os.RemoveAll(repoDir)
	}
	return nil
}

// VCS Initialization Helper
func initSystemRepo(sys, repoDir, gitBin, twigBin, dataset string) error {
	switch sys {
	case "git":
		if err := runCommandInDir(repoDir, gitBin, "init"); err != nil {
			return err
		}
		_ = runCommandInDir(repoDir, gitBin, "config", "user.name", "Bench Runner")
		_ = runCommandInDir(repoDir, gitBin, "config", "user.email", "bench@example.com")

	case "lfs":
		if err := runCommandInDir(repoDir, gitBin, "init"); err != nil {
			return err
		}
		_ = runCommandInDir(repoDir, gitBin, "config", "user.name", "Bench Runner")
		_ = runCommandInDir(repoDir, gitBin, "config", "user.email", "bench@example.com")

		if err := runCommandInDir(repoDir, gitBin, "lfs", "install"); err != nil {
			return fmt.Errorf("failed to run git lfs install: %w", err)
		}

		// Set tracking filter based on dataset type
		filter := "*.bin"
		switch dataset {
		case "sqlite":
			filter = "sqlite.db"
		case "jpegs":
			filter = "*.jpg"
		}

		if err := runCommandInDir(repoDir, gitBin, "lfs", "track", filter); err != nil {
			return fmt.Errorf("failed to track %s in LFS: %w", filter, err)
		}

		// Stage and commit the .gitattributes file before adding dataset files
		if err := runCommandInDir(repoDir, gitBin, "add", ".gitattributes"); err != nil {
			return err
		}
		if err := runCommandInDir(repoDir, gitBin, "commit", "-m", "Initialize Git LFS tracking"); err != nil {
			return err
		}

	case "twig":
		if err := runCommandInDir(repoDir, twigBin, "init"); err != nil {
			return err
		}
		// Write author config manually
		configPath := filepath.Join(repoDir, ".twig", "config")
		if err := os.WriteFile(configPath, []byte("user.id=bench-runner\n"), 0644); err != nil {
			return fmt.Errorf("failed to configure twig author: %w", err)
		}
	}
	return nil
}

// Stage and Commit Execution Helper
func runAddAndCommit(sys, repoDir, gitBin, twigBin string, rev int) (time.Duration, error) {
	msg := fmt.Sprintf("revision %d", rev)
	start := time.Now()

	switch sys {
	case "git", "lfs":
		if err := runCommandInDir(repoDir, gitBin, "add", "."); err != nil {
			return 0, err
		}
		if err := runCommandInDir(repoDir, gitBin, "commit", "-m", msg); err != nil {
			return 0, err
		}
	case "twig":
		if err := runCommandInDir(repoDir, twigBin, "add", "."); err != nil {
			return 0, err
		}
		if err := runCommandInDir(repoDir, twigBin, "commit", "-m", msg); err != nil {
			return 0, err
		}
	}

	return time.Since(start), nil
}

// Checkout Speed Helper
func runCheckoutBenchmark(sys, repoDir, gitBin, twigBin string) (time.Duration, error) {
	checkoutTargetDir := filepath.Join(".out", fmt.Sprintf("bench_checkout_%s", sys))
	_ = os.RemoveAll(checkoutTargetDir)
	if err := os.MkdirAll(checkoutTargetDir, 0755); err != nil {
		return 0, err
	}
	defer os.RemoveAll(checkoutTargetDir)

	// Copy control/metadata folders
	metaDirName := ".git"
	if sys == "twig" {
		metaDirName = ".twig"
	}
	srcMeta := filepath.Join(repoDir, metaDirName)
	dstMeta := filepath.Join(checkoutTargetDir, metaDirName)
	if err := copyDirectoryRecursive(srcMeta, dstMeta); err != nil {
		return 0, fmt.Errorf("failed to copy metadata: %w", err)
	}

	// For Git/LFS, copy .gitattributes if it exists in source
	switch sys {
	case "lfs":
		srcAttr := filepath.Join(repoDir, ".gitattributes")
		dstAttr := filepath.Join(checkoutTargetDir, ".gitattributes")
		if _, err := os.Stat(srcAttr); err == nil {
			_ = copyFile(srcAttr, dstAttr)
		}
		// Git LFS needs LFS configs or local repository variables set
		_ = runCommandInDir(checkoutTargetDir, gitBin, "config", "user.name", "Bench Runner")
		_ = runCommandInDir(checkoutTargetDir, gitBin, "config", "user.email", "bench@example.com")
	case "git":
		_ = runCommandInDir(checkoutTargetDir, gitBin, "config", "user.name", "Bench Runner")
		_ = runCommandInDir(checkoutTargetDir, gitBin, "config", "user.email", "bench@example.com")
	}

	// Run checkout and measure duration
	start := time.Now()
	switch sys {
	case "git", "lfs":
		if err := runCommandInDir(checkoutTargetDir, gitBin, "checkout", "-f", "HEAD"); err != nil {
			return 0, err
		}
	case "twig":
		if err := runCommandInDir(checkoutTargetDir, twigBin, "checkout", "--force", "main"); err != nil {
			return 0, err
		}
	}
	return time.Since(start), nil
}

// Helpers

func checkGitVersion(gitBin string) error {
	cmd := exec.Command(gitBin, "--version")
	return cmd.Run()
}

func findTwigBinary(customPath string) (string, error) {
	if customPath != "" {
		if _, err := os.Stat(customPath); err == nil {
			return filepath.Abs(customPath)
		}
		return "", fmt.Errorf("custom path %q not found", customPath)
	}

	cwd, err := os.Getwd()
	if err == nil {
		path := filepath.Join(cwd, "twig.exe")
		if _, err := os.Stat(path); err == nil {
			return filepath.Abs(path)
		}
		pathUnix := filepath.Join(cwd, "twig")
		if _, err := os.Stat(pathUnix); err == nil {
			return filepath.Abs(pathUnix)
		}
	}

	// Fallback to system PATH
	return "twig", nil
}

func runCommandInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	// Silence outputs unless error
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmd %s %v failed: %w, stderr: %s", name, args, err, stderrBuf.String())
	}
	return nil
}

func cleanRepoWorkingDir(repoDir string) error {
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if name == ".git" || name == ".twig" || name == ".gitattributes" {
			continue
		}
		path := filepath.Join(repoDir, name)
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func copyDirectoryContent(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirectoryRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyDirectoryRecursive(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func calculateDirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func writeBenchmarkCSV(path string, results []BenchmarkResult, checkoutDuration time.Duration) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.WriteString(f, "revision,add_commit_seconds,cumulative_repo_size_bytes\n"); err != nil {
		return err
	}

	for _, r := range results {
		line := fmt.Sprintf("%d,%.3f,%d\n", r.Revision, r.Duration.Seconds(), r.FolderSize)
		if _, err := io.WriteString(f, line); err != nil {
			return err
		}
	}

	checkoutLine := fmt.Sprintf("checkout,%.3f,0\n", checkoutDuration.Seconds())
	_, err = io.WriteString(f, checkoutLine)
	return err
}
