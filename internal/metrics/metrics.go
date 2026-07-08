package metrics

import (
	"os"
	"sync/atomic"
)

// Enabled indicates whether metrics tracking is active, governed by the TWIG_METRICS environment variable.
var Enabled bool

var (
	StorePutCalls      atomic.Int64
	StorePutDedupSkips atomic.Int64
	ChunkerInvocations atomic.Int64
	HashFileCalls      atomic.Int64
)

func init() {
	Enabled = os.Getenv("TWIG_METRICS") == "1"
}

// Snapshot returns a copy of all counter values as a map.
func Snapshot() map[string]int64 {
	return map[string]int64{
		"store_put_calls":       StorePutCalls.Load(),
		"store_put_dedup_skips": StorePutDedupSkips.Load(),
		"chunker_invocations":  ChunkerInvocations.Load(),
		"hash_file_calls":       HashFileCalls.Load(),
	}
}
