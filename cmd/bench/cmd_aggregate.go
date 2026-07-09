package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SystemSummary struct {
	Dataset      string
	System       string
	FinalSize    int64
	TotalCommit  float64
	CheckoutTime float64
}

// aggregateResults reads raw CSV files and generates the markdown summary.md.
func aggregateResults(rawDir, summaryPath string) error {
	datasets := []string{"sqlite", "assets", "jpegs"}
	systems := []string{"git", "lfs", "twig"}

	var summaries []SystemSummary

	for _, ds := range datasets {
		for _, sys := range systems {
			csvName := fmt.Sprintf("%s-%s.csv", ds, sys)
			csvPath := filepath.Join(rawDir, csvName)

			summary, err := parseCSVResults(ds, sys, csvPath)
			if err != nil {
				// If a file is missing, we skip it but warn the user.
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", csvName, err)
				continue
			}
			summaries = append(summaries, summary)
		}
	}

	if len(summaries) == 0 {
		return fmt.Errorf("no CSV results found under %s to aggregate", rawDir)
	}

	// Generate summary markdown table content
	var sb strings.Builder
	sb.WriteString("# Twig VCS — Performance Benchmark Summary\n\n")
	sb.WriteString("This file summarizes the benchmark execution metrics comparing standard Git, Git LFS, and Twig across structured revision snapshots. All timings are in seconds and sizing metrics are in megabytes (MB).\n\n")
	sb.WriteString("| Dataset | System | Final Repo Size | Total Add+Commit Time | Checkout Time |\n")
	sb.WriteString("|---|---|---|---|---|\n")

	for _, s := range summaries {
		sizeMB := float64(s.FinalSize) / (1024 * 1024)
		sb.WriteString(fmt.Sprintf("| %s | %s | %.2f MB | %.3f s | %.3f s |\n",
			strings.Title(s.Dataset),
			strings.ToUpper(s.System),
			sizeMB,
			s.TotalCommit,
			s.CheckoutTime,
		))
	}

	// Write markdown file
	summaryDir := filepath.Dir(summaryPath)
	if err := os.MkdirAll(summaryDir, 0755); err != nil {
		return fmt.Errorf("failed to create summary directory: %w", err)
	}

	if err := os.WriteFile(summaryPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write summary file: %w", err)
	}

	return nil
}

func parseCSVResults(dataset, system, csvPath string) (SystemSummary, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return SystemSummary{}, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	// Read header
	_, err = reader.Read()
	if err != nil {
		return SystemSummary{}, err
	}

	var totalCommitTime float64
	var finalSize int64
	var checkoutTime float64

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return SystemSummary{}, err
		}

		if len(record) < 3 {
			continue
		}

		rowType := record[0]
		if rowType == "checkout" {
			t, _ := strconv.ParseFloat(record[1], 64)
			checkoutTime = t
		} else {
			// It is a revision number
			t, _ := strconv.ParseFloat(record[1], 64)
			totalCommitTime += t

			sz, _ := strconv.ParseInt(record[2], 10, 64)
			finalSize = sz // The last size read will represent the final size
		}
	}

	return SystemSummary{
		Dataset:      dataset,
		System:       system,
		FinalSize:    finalSize,
		TotalCommit:  totalCommitTime,
		CheckoutTime: checkoutTime,
	}, nil
}
