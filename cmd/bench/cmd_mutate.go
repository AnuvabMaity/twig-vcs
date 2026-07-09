package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
)

// runMutate implements 'bench mutate'
func runMutate(args []string) {
	fs := flag.NewFlagSet("mutate", flag.ExitOnError)
	mode := fs.String("mode", "", "Mutation mode: append, insert, delete, flip (required)")
	offset := fs.Int("offset", -1, "Byte offset (optional, defaults to middle of file)")
	length := fs.Int("length", 100, "Length of bytes to mutate")
	seed := fs.Int64("seed", 42, "Random seed")

	var filePath string
	var parseArgs []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		filePath = args[0]
		parseArgs = args[1:]
	} else {
		parseArgs = args
	}

	if err := fs.Parse(parseArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if filePath == "" {
		extraArgs := fs.Args()
		if len(extraArgs) > 0 {
			filePath = extraArgs[0]
		}
	}

	if filePath == "" {
		fmt.Fprintln(os.Stderr, "Usage: bench mutate <file> --mode <append|insert|delete|flip> [--offset N] [--length N] [--seed N]")
		os.Exit(1)
	}

	if *mode == "" {
		fmt.Fprintln(os.Stderr, "Error: --mode is required")
		os.Exit(1)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	oldSize := len(content)
	off := *offset
	if off < 0 {
		off = oldSize / 2
	}
	if off > oldSize {
		off = oldSize
	}

	rng := rand.New(rand.NewSource(*seed))
	var newContent []byte

	switch *mode {
	case "append":
		randomBytes := make([]byte, *length)
		rng.Read(randomBytes)
		newContent = append(content, randomBytes...)
		fmt.Printf("Appended %d random bytes to the end of %s.\n", *length, filePath)

	case "insert":
		randomBytes := make([]byte, *length)
		rng.Read(randomBytes)
		newContent = make([]byte, 0, oldSize+*length)
		newContent = append(newContent, content[:off]...)
		newContent = append(newContent, randomBytes...)
		newContent = append(newContent, content[off:]...)
		fmt.Printf("Inserted %d random bytes at offset %d in %s.\n", *length, off, filePath)

	case "delete":
		deleteLen := *length
		if off+deleteLen > oldSize {
			deleteLen = oldSize - off
		}
		newContent = make([]byte, 0, oldSize-deleteLen)
		newContent = append(newContent, content[:off]...)
		newContent = append(newContent, content[off+deleteLen:]...)
		fmt.Printf("Deleted %d bytes starting at offset %d in %s.\n", deleteLen, off, filePath)

	case "flip":
		flipLen := *length
		if off+flipLen > oldSize {
			flipLen = oldSize - off
		}
		randomBytes := make([]byte, flipLen)
		rng.Read(randomBytes)
		newContent = make([]byte, oldSize)
		copy(newContent, content)
		copy(newContent[off:off+flipLen], randomBytes)
		fmt.Printf("Flipped %d bytes starting at offset %d in %s.\n", flipLen, off, filePath)

	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s. Use append, insert, delete, or flip.\n", *mode)
		os.Exit(1)
	}

	if err := os.WriteFile(filePath, newContent, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing mutated file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	fmt.Printf("File size changed from %d to %d bytes.\n", oldSize, len(newContent))
}
