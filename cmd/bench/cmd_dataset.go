package main

import (
	"database/sql"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// runGenDataset implements 'bench gen dataset'
func runGenDataset(args []string) {
	fs := flag.NewFlagSet("gen dataset", flag.ExitOnError)
	revisions := fs.Int("revisions", 20, "Number of revisions to generate")
	outDir := fs.String("out", "", "Directory to place the revisions (required)")
	seed := fs.Int64("seed", 42, "Random seed")

	var datasetType string
	var parseArgs []string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		datasetType = args[0]
		parseArgs = args[1:]
	} else {
		parseArgs = args
	}

	if err := fs.Parse(parseArgs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if datasetType == "" {
		extraArgs := fs.Args()
		if len(extraArgs) > 0 {
			datasetType = extraArgs[0]
		}
	}

	if datasetType == "" {
		fmt.Fprintln(os.Stderr, "Usage: bench gen dataset <sqlite|assets|jpegs> --out DIR [--revisions N] [--seed N]")
		os.Exit(1)
	}

	if *outDir == "" {
		fmt.Fprintln(os.Stderr, "Error: --out directory is required")
		os.Exit(1)
	}

	if err := generateDatasetInternal(datasetType, *outDir, *revisions, *seed); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating dataset: %v\n", err)
		os.Exit(1)
	}
}

// generateDatasetInternal encapsulates dataset generation logic for programmatic invocation.
func generateDatasetInternal(datasetType string, outDir string, revisions int, seed int64) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	rng := rand.New(rand.NewSource(seed))

	switch datasetType {
	case "sqlite":
		fmt.Printf("Generating SQLite dataset with %d revisions...\n", revisions)
		tempDBPath := filepath.Join(outDir, "temp_sqlite.db")
		// Clean up any stray temp db
		os.Remove(tempDBPath)

		for i := 1; i <= revisions; i++ {
			db, err := sql.Open("sqlite", tempDBPath)
			if err != nil {
				return fmt.Errorf("failed to open SQLite database: %w", err)
			}

			_, err = db.Exec("CREATE TABLE IF NOT EXISTS dataset (id INTEGER PRIMARY KEY, rev INTEGER, key TEXT, payload TEXT)")
			if err != nil {
				db.Close()
				return fmt.Errorf("failed to create table: %w", err)
			}

			// Insert 50 rows per revision
			for r := 0; r < 50; r++ {
				_, err = db.Exec("INSERT INTO dataset (rev, key, payload) VALUES (?, ?, ?)",
					i,
					fmt.Sprintf("key_rev%d_%d", i, r),
					randString(rng, 300),
				)
				if err != nil {
					db.Close()
					return fmt.Errorf("failed to insert rows: %w", err)
				}
			}

			db.Close() // Close database to flush journal and finalize file layout

			revDir := filepath.Join(outDir, "revisions", fmt.Sprintf("rev-%02d", i))
			if err := os.MkdirAll(revDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", revDir, err)
			}

			destDBPath := filepath.Join(revDir, "sqlite.db")
			if err := copyFile(tempDBPath, destDBPath); err != nil {
				return fmt.Errorf("failed to snapshot SQLite database: %w", err)
			}
			fmt.Printf("  Snapshot created: %s\n", destDBPath)
		}
		os.Remove(tempDBPath)
		fmt.Println("SQLite dataset generated successfully.")

	case "assets":
		fmt.Printf("Generating Assets dataset with %d revisions...\n", revisions)
		// 3 binary files representing asset templates (total size ~7.5MB)
		sizes := []int{1200 * 1024, 2500 * 1024, 3800 * 1024}
		names := []string{"texture_1.bin", "model_head.bin", "audio_voice.bin"}
		contents := make([][]byte, 3)

		for idx, sz := range sizes {
			contents[idx] = generateRedundantBytes(rng, sz)
		}

		for i := 1; i <= revisions; i++ {
			revDir := filepath.Join(outDir, "revisions", fmt.Sprintf("rev-%02d", i))
			if err := os.MkdirAll(revDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", revDir, err)
			}

			// In revision 1, write initial contents.
			// In revision > 1, apply a small mutation to one file compared to the previous revision.
			if i > 1 {
				targetFileIdx := rng.Intn(len(names))
				mutatedData := make([]byte, len(contents[targetFileIdx]))
				copy(mutatedData, contents[targetFileIdx])

				// Flip 100 bytes at a random offset
				lengthToMutate := 100
				offset := rng.Intn(len(mutatedData) - lengthToMutate)
				randomBytes := make([]byte, lengthToMutate)
				rng.Read(randomBytes)
				copy(mutatedData[offset:offset+lengthToMutate], randomBytes)

				contents[targetFileIdx] = mutatedData
			}

			for idx, name := range names {
				destPath := filepath.Join(revDir, name)
				if err := os.WriteFile(destPath, contents[idx], 0644); err != nil {
					return fmt.Errorf("failed to write asset %s: %w", destPath, err)
				}
			}
			fmt.Printf("  Snapshot created: %s/\n", revDir)
		}
		fmt.Println("Assets dataset generated successfully.")

	case "jpegs":
		fmt.Printf("Generating JPEGs dataset with %d revisions...\n", revisions)
		// Programmatically render 3 JPEG images and apply a small visual edit (adding text / lines) per revision
		names := []string{"photo_1.jpg", "diagram_arch.jpg", "preview_render.jpg"}
		bgColors := []color.RGBA{
			{R: 200, G: 220, B: 240, A: 255}, // light blue
			{R: 240, G: 240, B: 220, A: 255}, // light yellow
			{R: 220, G: 240, B: 220, A: 255}, // light green
		}

		for i := 1; i <= revisions; i++ {
			revDir := filepath.Join(outDir, "revisions", fmt.Sprintf("rev-%02d", i))
			if err := os.MkdirAll(revDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", revDir, err)
			}

			for idx, name := range names {
				img := image.NewRGBA(image.Rect(0, 0, 800, 600))
				draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColors[idx]}, image.Point{}, draw.Src)

				// Draw base random shapes
				drawRandomShapes(rng, img, 5)

				// Draw revision text visual change to make the images distinct and evolving
				// Draw simple colored blocks to represent text / visual revisions
				// In subsequent revisions, draw extra blocks or change their coordinates
				for r := 1; r <= i; r++ {
					// Draw a distinct visual block for each revision up to current
					x := 50 + (r-1)*30
					y := 50 + (idx * 100)
					rect := image.Rect(x, y, x+20, y+20)
					blockColor := color.RGBA{R: uint8(10 * r), G: uint8(200 - 5*r), B: uint8(50 + 10*r), A: 255}
					draw.Draw(img, rect, &image.Uniform{C: blockColor}, image.Point{}, draw.Src)
				}

				destPath := filepath.Join(revDir, name)
				f, err := os.Create(destPath)
				if err != nil {
					return fmt.Errorf("failed to create jpeg file: %w", err)
				}

				if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
					f.Close()
					return fmt.Errorf("failed to encode jpeg: %w", err)
				}
				f.Close()
			}
			fmt.Printf("  Snapshot created: %s/\n", revDir)
		}
		fmt.Println("JPEGs dataset generated successfully.")

	default:
		return fmt.Errorf("unknown dataset type: %s. Use sqlite, assets, or jpegs", datasetType)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func drawRandomShapes(rng *rand.Rand, img *image.RGBA, count int) {
	for i := 0; i < count; i++ {
		x1 := rng.Intn(800)
		y1 := rng.Intn(600)
		x2 := x1 + rng.Intn(100) + 10
		y2 := y1 + rng.Intn(100) + 10
		rect := image.Rect(x1, y1, x2, y2)
		c := color.RGBA{R: uint8(rng.Intn(256)), G: uint8(rng.Intn(256)), B: uint8(rng.Intn(256)), A: 255}
		draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
	}
}
