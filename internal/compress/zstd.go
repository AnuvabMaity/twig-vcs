package compress

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
)

var (
	encoder *zstd.Encoder
	decoder *zstd.Decoder
)

func init() {
	var err error
	encoder, err = zstd.NewWriter(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd encoder: %v", err))
	}

	decoder, err = zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("failed to create zstd decoder: %v", err))
	}
}

// Compress compresses data using zstd.
func Compress(data []byte) ([]byte, error) {
	return encoder.EncodeAll(data, nil), nil
}

// Decompress decompresses data using zstd.
// Returns an error if the data is corrupted or invalid.
func Decompress(data []byte) ([]byte, error) {
	return decoder.DecodeAll(data, nil)
}
