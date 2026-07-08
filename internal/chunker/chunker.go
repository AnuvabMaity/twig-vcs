package chunker

import (
	"io"

	"github.com/jotfs/fastcdc-go"
	"twig/internal/metrics"
	"twig/internal/objects"
)

// Split reads all of r and splits it into content-defined chunks using
// the project's configured min/avg/max sizes. Chunk order matches the
// order chunks appear in the source data.
// Splitting an empty input does not panic and returns zero chunks.
func Split(r io.Reader) ([][]byte, error) {
	if metrics.Enabled {
		metrics.ChunkerInvocations.Add(1)
	}
	opts := fastcdc.Options{
		MinSize:     objects.ChunkMinSize,
		AverageSize: objects.ChunkAvgSize,
		MaxSize:     objects.ChunkMaxSize,
	}

	chunker, err := fastcdc.NewChunker(r, opts)
	if err != nil {
		return nil, err
	}

	var chunks [][]byte
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		// Copy the chunk data since the chunker may reuse its underlying buffer.
		chunkCopy := make([]byte, len(chunk.Data))
		copy(chunkCopy, chunk.Data)
		chunks = append(chunks, chunkCopy)
	}

	return chunks, nil
}
