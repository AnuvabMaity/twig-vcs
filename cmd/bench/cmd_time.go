package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

// runBenchTime implements 'bench time [--label NAME] -- <command...>'
func runBenchTime(args []string) {
	// Parse wrapper flags up to "--"
	var wrapperArgs []string
	var cmdArgs []string

	for i, arg := range args {
		if arg == "--" {
			wrapperArgs = args[:i]
			cmdArgs = args[i+1:]
			break
		}
	}

	// If no "--" was provided, print usage and exit
	if len(cmdArgs) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: bench time [--label NAME] -- <command...>")
		fmt.Fprintln(os.Stderr, "Example: bench time --label \"commit-large\" -- twig commit -m \"large upload\"")
		os.Exit(1)
	}

	fs := flag.NewFlagSet("time", flag.ExitOnError)
	label := fs.String("label", "unnamed", "Label for this timing run")
	if err := fs.Parse(wrapperArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Prepare subprocess command
	// Convert forward slashes to backslashes on Windows (e.g., ./twig.exe -> .\twig.exe)
	cmdPath := filepath.FromSlash(cmdArgs[0])
	subCmd := exec.Command(cmdPath, cmdArgs[1:]...)
	subCmd.Stdout = os.Stdout

	// Capture stderr to extract metrics dump, while still streaming it to the parent's stderr
	var stderrBuf bytes.Buffer
	subCmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	// Inject metrics environment variable
	subCmd.Env = append(os.Environ(), "TWIG_METRICS=1")

	// Execute process
	startTime := time.Now()
	runErr := subCmd.Run()
	duration := time.Since(startTime)

	// Collect peak RSS if available using reflection
	var peakRSS int64
	if subCmd.ProcessState != nil {
		getPeakRSS(subCmd.ProcessState, &peakRSS)
	}

	// Extract metrics JSON line from stderr buffer
	metricsData := make(map[string]int64)
	stderrStr := stderrBuf.String()
	marker := "TWIG_METRICS_DUMP:"
	if _, after, ok := strings.Cut(stderrStr, marker); ok {
		line := after
		if endIdx := strings.Index(line, "\n"); endIdx != -1 {
			line = line[:endIdx]
		}
		line = strings.TrimSpace(line)
		_ = json.Unmarshal([]byte(line), &metricsData)
	}

	// Output terminal report
	fmt.Println()
	fmt.Println("--- Performance Timing Summary ---")
	fmt.Printf("Label:    %s\n", *label)
	fmt.Printf("Command:  %s\n", strings.Join(cmdArgs, " "))
	fmt.Printf("Duration: %s (%.2f seconds)\n", duration, duration.Seconds())
	if peakRSS > 0 {
		fmt.Printf("Peak RSS: %.2f MB (%d bytes)\n", float64(peakRSS)/(1024*1024), peakRSS)
	}
	if len(metricsData) > 0 {
		fmt.Println("Internal Counters:")
		fmt.Printf("  └─ Chunker Invocations:   %d\n", metricsData["chunker_invocations"])
		fmt.Printf("  └─ Store Put Calls:       %d\n", metricsData["store_put_calls"])
		fmt.Printf("  └─ Store Put Dedup Skips: %d\n", metricsData["store_put_dedup_skips"])
		fmt.Printf("  └─ Hash File Calls:       %d\n", metricsData["hash_file_calls"])
	}
	fmt.Println("----------------------------------")

	// Append results to bench/results/timings.csv
	resultsDir := filepath.Join("bench", "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create results directory: %v\n", err)
	} else {
		csvPath := filepath.Join(resultsDir, "timings.csv")
		fileExisted := true
		if _, err := os.Stat(csvPath); os.IsNotExist(err) {
			fileExisted = false
		}

		f, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write to timings.csv: %v\n", err)
		} else {
			defer f.Close()
			if !fileExisted {
				// Write CSV header
				fmt.Fprintln(f, "timestamp,label,command,duration_ms,peak_rss_bytes,chunker_invocations,store_put_calls,store_put_dedup_skips,hash_file_calls")
			}
			// Write CSV row
			fmt.Fprintf(f, "%s,%s,%s,%d,%d,%d,%d,%d,%d\n",
				time.Now().Format(time.RFC3339),
				*label,
				strings.ReplaceAll(strings.Join(cmdArgs, " "), ",", " "),
				duration.Milliseconds(),
				peakRSS,
				metricsData["chunker_invocations"],
				metricsData["store_put_calls"],
				metricsData["store_put_dedup_skips"],
				metricsData["hash_file_calls"],
			)
		}
	}

	// Exit with the same status code as the sub-command
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error: failed to start command %q: %v\n", cmdArgs[0], runErr)
		os.Exit(1)
	}
}

func getPeakRSS(ps *os.ProcessState, peakRSS *int64) {
	sysUsage := ps.SysUsage()
	if sysUsage == nil {
		return
	}

	val := reflect.ValueOf(sysUsage)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Struct {
		maxrssField := val.FieldByName("Maxrss")
		if maxrssField.IsValid() && maxrssField.CanInt() {
			rss := maxrssField.Int()
			// On Darwin (macOS), maxrss is returned in bytes.
			// On Linux/BSD, maxrss is returned in kilobytes.
			if runtime.GOOS == "darwin" {
				*peakRSS = rss
			} else {
				*peakRSS = rss * 1024
			}
		}
	}
}
