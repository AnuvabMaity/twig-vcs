package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"twig/internal/index"
	"twig/internal/objects"
	"twig/internal/store"

	"github.com/fxamacker/cbor/v2"
)

// runCatIndex implements 'bench cat index'
func runCatIndex(args []string) {
	fs := flag.NewFlagSet("cat index", flag.ExitOnError)
	storeDir := fs.String("store", "", "Path to the .twig directory (optional, auto-discovered if empty)")
	jsonOut := fs.Bool("json", false, "Output in raw JSON format")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	var twigDir string
	var err error
	if *storeDir != "" {
		twigDir = *storeDir
	} else {
		twigDir, err = FindTwigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering repository: %v\n", err)
			os.Exit(1)
		}
	}

	indexPath := filepath.Join(twigDir, objects.IndexFileName)
	idx, err := index.Load(indexPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading index: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		// Output staging index as JSON
		jsonData, err := json.MarshalIndent(idx.Entries, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
		return
	}

	// Human-readable output
	if len(idx.Entries) == 0 {
		fmt.Println("Staging index is empty.")
		return
	}

	fmt.Printf("Staging Index (%d files):\n", len(idx.Entries))
	fmt.Printf("%-35s | %-6s | %-10s | %-20s | %-12s\n", "Path", "Type", "Size", "Mod Time", "Conflict")
	fmt.Println(string(make([]byte, 95))) // separation line
	for path, entry := range idx.Entries {
		t := time.Unix(0, entry.ModTime).Format(time.RFC3339)
		conflictStr := "No"
		if entry.Conflict != nil {
			conflictStr = "YES"
		}
		fmt.Printf("%-35s | %-6s | %-10d | %-20s | %-12s\n", path, entry.Type, entry.Size, t, conflictStr)
		if entry.Conflict != nil {
			fmt.Printf("  └─ ours:   %s (%s)\n", entry.Conflict.OursHash, entry.Conflict.OursType)
			fmt.Printf("  └─ theirs: %s (%s)\n", entry.Conflict.TheirsHash, entry.Conflict.TheirsType)
		}
	}
}

// runCatObject implements 'bench cat object <ref-or-hash>'
func runCatObject(args []string) {
	fs := flag.NewFlagSet("cat object", flag.ExitOnError)
	storeDir := fs.String("store", "", "Path to the .twig directory (optional, auto-discovered if empty)")
	jsonOut := fs.Bool("json", false, "Output in raw JSON format")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	extraArgs := fs.Args()
	if len(extraArgs) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: bench cat object <ref-or-hash> [--store DIR] [--json]")
		os.Exit(1)
	}

	targetRef := extraArgs[0]

	var twigDir string
	var err error
	if *storeDir != "" {
		twigDir = *storeDir
	} else {
		twigDir, err = FindTwigDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error discovering repository: %v\n", err)
			os.Exit(1)
		}
	}

	hash, err := ResolveRefOrHash(twigDir, targetRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving reference %q: %v\n", targetRef, err)
		os.Exit(1)
	}

	st := store.Open(twigDir)
	data, err := st.Get(hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading object %s: %v\n", hash, err)
		os.Exit(1)
	}

	// Auto-detect type
	var header struct {
		Type string `cbor:"type"`
	}
	objType := "chunk"
	if err := objects.Decode(data, &header); err == nil && header.Type != "" {
		objType = header.Type
	}

	if *jsonOut {
		if objType == "chunk" {
			// A raw chunk is binary, format as JSON holding metadata and hex preview
			chunkMeta := map[string]interface{}{
				"type": "chunk",
				"size": len(data),
				"hash": hash,
				"hex":  hex.EncodeToString(data[:min(len(data), 128)]),
			}
			jsonData, _ := json.MarshalIndent(chunkMeta, "", "  ")
			fmt.Println(string(jsonData))
			return
		}

		// Decode structured CBOR to generic interface{} and convert interface{} keys to string
		var raw interface{}
		if err := cbor.Unmarshal(data, &raw); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing CBOR: %v\n", err)
			os.Exit(1)
		}
		converted := convertKeysToStrings(raw)
		jsonData, err := json.MarshalIndent(converted, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
		return
	}

	// Human-readable presentation
	fmt.Printf("Object: %s\n", hash)
	fmt.Printf("Type:   %s\n", objType)
	fmt.Printf("Size:   %d bytes\n", len(data))
	fmt.Println(string(make([]byte, 75)))

	switch objType {
	case "commit":
		var c objects.Commit
		if err := objects.Decode(data, &c); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding commit: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Tree:      %s\n", c.Root)
		fmt.Printf("Parents:   %v\n", c.Parents)
		fmt.Printf("Author ID: %s\n", c.Author.ID)
		fmt.Printf("Time:      %s\n", time.Unix(c.Author.Time, 0).Format(time.RFC3339))
		fmt.Println()
		fmt.Println("Message:")
		fmt.Printf("  %s\n", c.Message)

	case "tree":
		var t objects.Tree
		if err := objects.Decode(data, &t); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding tree: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Entries in Tree (%d):\n", len(t.Entries))
		fmt.Printf("%-6s | %-64s | %s\n", "Type", "Hash", "Name")
		fmt.Println(string(make([]byte, 75)))
		for _, e := range t.Entries {
			fmt.Printf("%-6s | %-64s | %s\n", e.Type, e.Hash, e.Name)
		}

	case "blob":
		var b objects.Blob
		if err := objects.Decode(data, &b); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding blob: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Content (first 1024 bytes shown for safety):")
		fmt.Println(string(make([]byte, 75)))
		previewLen := min(len(b.Data), 1024)
		// Check if it looks printable
		isPrintable := true
		for _, char := range b.Data[:previewLen] {
			if char < 32 && char != '\n' && char != '\r' && char != '\t' {
				isPrintable = false
				break
			}
		}
		if isPrintable {
			fmt.Println(string(b.Data[:previewLen]))
		} else {
			fmt.Println(hex.Dump(b.Data[:previewLen]))
		}
		if len(b.Data) > 1024 {
			fmt.Printf("\n... [truncated, %d bytes remaining] ...\n", len(b.Data)-1024)
		}

	case "asset":
		var a objects.Asset
		if err := objects.Decode(data, &a); err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding asset: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Logical size: %d bytes\n", a.Size)
		fmt.Printf("Chunks count: %d\n", len(a.Chunks))
		fmt.Println()
		fmt.Println("Chunks list:")
		fmt.Printf("%-3s | %-64s | %-10s\n", "Idx", "Hash", "Size")
		fmt.Println(string(make([]byte, 75)))
		for idx, c := range a.Chunks {
			fmt.Printf("%-3d | %-64s | %-10d\n", idx, c.Hash, c.Size)
		}

	case "chunk":
		fmt.Println("Raw Chunk Content (first 512 bytes):")
		fmt.Println(string(make([]byte, 75)))
		previewLen := min(len(data), 512)
		fmt.Println(hex.Dump(data[:previewLen]))
		if len(data) > 512 {
			fmt.Printf("\n... [truncated, %d bytes remaining] ...\n", len(data)-512)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// convertKeysToStrings recursively converts map[interface{}]interface{} keys to strings.
func convertKeysToStrings(val interface{}) interface{} {
	switch x := val.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range x {
			m[fmt.Sprint(k)] = convertKeysToStrings(v)
		}
		return m
	case []interface{}:
		s := make([]interface{}, len(x))
		for i, v := range x {
			s[i] = convertKeysToStrings(v)
		}
		return s
	default:
		return x
	}
}
