# Twig VCS

A content-addressable version control system built in Go, designed around **Content-Defined Chunking (CDC)** for efficient storage of large binary files across revisions. Twig provides a Git-like workflow — `init`, `add`, `commit`, `log`, `checkout`, `branch`, `merge` — while storing file data as chunk-deduplicated loose objects rather than git-style packfiles.

---

## Table of Contents

- [Design Rationale](#design-rationale)
- [Architecture Overview](#architecture-overview)
- [Object Model](#object-model)
- [On-Disk Layout](#on-disk-layout)
- [Key Architectural Constants](#key-architectural-constants)
- [Package Structure](#package-structure)
- [Building & Running](#building--running)
- [Command Reference](#command-reference)
- [Configuration](#configuration)
- [Merge Strategy](#merge-strategy)
- [Instrumentation](#instrumentation)
- [Design Tradeoffs](#design-tradeoffs)
- [Known Limitations & Future Work](#known-limitations--future-work)
- [Dependencies](#dependencies)

---

## Design Rationale

Standard VCS tools handle binary files poorly. Git stores each revision of a binary asset as a separate compressed blob; Git LFS improves storage but requires external infrastructure. Neither exploits the internal structure of evolving binary files.

Twig uses **FastCDC** (content-defined chunking) to split files into variable-size chunks at content-dependent boundaries. When a file changes, only the chunks whose byte content actually changed need to be written. Chunks whose boundaries and content are unaffected are automatically deduplicated at write time via content addressing.

This makes Twig a reasonable fit for repositories that track iteratively-modified binary assets — SQLite databases, game assets, or similar append-heavy workloads. For repositories dominated by small text files or highly compressed data (e.g., JPEGs, MP4s), the deduplication benefit is negligible and Twig offers no advantage over Git.

---

## Architecture Overview

```
Working Directory
       │
       │  twig add
       ▼
  Staging Index  (.twig/index)
   (CBOR map: path → {hash, type, size, mtime})
       │
       │  twig commit
       ▼
  Tree Objects  ──►  Commit Objects
  (.twig/objects/)    (.twig/objects/)
       │                    │
       └──────┬─────────────┘
              ▼
    Loose Object Store
    (content-addressed, zstd-compressed)
    .twig/objects/<hash[:2]>/<hash[2:]>
              │
    ┌─────────┴──────────┐
    │                    │
  Blob               Asset + Chunks
  (< 16 KB)          (≥ 16 KB, FastCDC)
```

---

## Object Model

Twig defines four object types, stored in the same content-addressable object store.

- **Blob** — stores the raw content of a small file (< 16 KB) as a single atomic unit.
- **Asset** — stores a manifest of ordered chunk references for files ≥ 16 KB; content lives in the chunks, not the manifest itself.
- **Tree** — represents a directory snapshot as a sorted list of name-to-hash mappings, covering files and nested subdirectories.
- **Commit** — records a point-in-time snapshot: a root tree hash, zero or more parent commit hashes, author identity, timestamp, and message.

---

## On-Disk Layout

```
.twig/
├── HEAD           # symbolic ref ("ref: refs/heads/main") or detached commit hash
├── VERSION        # format version integer (currently "1")
├── config         # key=value repository configuration
├── index          # CBOR-encoded staging area
├── MERGE_HEAD     # present only during an active merge
├── objects/
│   └── <xx>/      # 2-character hex fan-out prefix
│       └── <yy…>  # remaining 62 hex chars; file content is zstd-compressed CBOR
└── refs/
    └── heads/
        └── <branch-name>  # plain text file containing the tip commit hash
```

---

## Key Architectural Constants

All of these are defined in a single location: `internal/objects/constants.go`. They must not be duplicated or overridden elsewhere.

| Constant | Value | Purpose |
|---|---|---|
| `BlobThreshold` | **16 KB** | Files below this are stored as Blobs; at or above as Assets |
| `ChunkMinSize` | **64 KB** | Minimum FastCDC chunk size |
| `ChunkAvgSize` | **256 KB** | Target average FastCDC chunk size |
| `ChunkMaxSize` | **1 MB** | Maximum FastCDC chunk size |
| `FormatVersion` | **1** | Written to `.twig/VERSION` on `init` for future migration detection |
| `DefaultBranchName` | `"main"` | Branch created on `twig init` |

The 64/256/1024 KB chunk parameters are a deliberate tradeoff: smaller averages improve dedup granularity at the cost of more objects and more small I/O operations; larger averages reduce object count but decrease dedup sensitivity. The 256 KB average is a reasonable middle ground for large binary files in the 1–500 MB range.

---

## Package Structure

```
twig/
├── cmd/
│   ├── twig/          # CLI entry point; one file per subcommand
│   └── bench/         # standalone benchmark harness (separate binary)
└── internal/
    ├── objects/        # type definitions, canonical CBOR codec, shared constants
    ├── hashing/        # BLAKE3 wrapper; hash-to-path fan-out derivation
    ├── compress/       # zstd encode/decode wrappers (singleton encoder/decoder)
    ├── store/          # loose object store: Put (dedup write), Get, Has
    ├── chunker/        # FastCDC wrapper with project-configured min/avg/max
    ├── ingest/         # Blob/Asset dispatch; file reconstruction (Reconstruct)
    ├── index/          # staging area: Load/Save/Put/Remove/NeedsRehash
    ├── refs/           # HEAD and branch ref read/write; ListBranches
    ├── repo/           # high-level orchestration: Add, Commit, Checkout, Status,
    │                   # Merge, CreateBranch, Log, ResolveConflict, AbortMerge
    ├── metrics/        # atomic counters; enabled via TWIG_METRICS=1
    └── smoketest/      # integration smoke tests
```

---

## Building & Running

**Prerequisites:** Go 1.21 or later.

```bash
# Clone and build
git clone https://github.com/AnuvabMaity/twig
cd twig
go build -o twig ./cmd/twig

# Run all tests
go test ./...

# Run the linter (requires golangci-lint in PATH)
golangci-lint run
```

The build produces a single self-contained binary with no runtime dependencies.

---

## Command Reference

### `twig init`

Initializes a new repository in the current directory. Creates the `.twig/` directory structure, an empty staging index, a default config, `HEAD` pointing at `main`, and a `VERSION` file.

```bash
twig init
```

---

### `twig add <path>`

Stages a file or directory into the index. Directories are walked recursively. Passing a path that no longer exists on disk stages its deletion.

```bash
twig add file.bin
twig add ./assets/
twig add .
```

---

### `twig commit -m "<message>"`

Records a snapshot of the staging index as a new commit and advances the current branch. No-ops cleanly if nothing has changed. Author identity is taken from `.twig/config`, falling back to the OS username.

```bash
twig commit -m "add initial dataset"
```

---

### `twig log [<ref>]`

Prints commit history from HEAD or a given branch name, full hash, or 7-character short hash.

```bash
twig log
twig log feature-branch
twig log a3f8b2c
```

---

### `twig status`

Shows the state of every tracked path relative to the staging index and the last commit.

```bash
twig status
```

Possible states per file:

| Status | Meaning |
|---|---|
| `staged-new` | Staged; not present in the last commit |
| `staged-modified` | Staged; differs from the last commit |
| `modified` | Staged, but working copy has changed again since staging |
| `deleted` | In the index but missing from disk |
| `untracked` | On disk but not staged |
| `conflict` | Unresolved merge conflict |
| `unmodified` | Identical in working copy, index, and last commit |

---

### `twig checkout <branch-or-commit>`

Restores the working directory to a given branch or commit. Accepts a branch name, full hash, or 7-character short hash. Refuses to overwrite locally modified files.

```bash
twig checkout main
twig checkout feature-branch
twig checkout a3f8b2c
```

---

### `twig branch [<name>]`

With no arguments, lists all branches with `*` marking the current one. With a name, creates a new branch pointing at the current HEAD commit.

```bash
twig branch              # list branches
twig branch feature-x    # create branch at HEAD
```

---

### `twig merge <branch>`

Merges the named branch into the current branch via a three-way diff. Non-conflicting changes are applied automatically; conflicting files are flagged and require manual resolution.

```bash
twig merge feature-x

# Abort an in-progress merge and restore HEAD state
twig merge --abort
```

---

### `twig resolve <ours|theirs> <file>`

Resolves a merge conflict on a specific file by accepting one side. After resolving all conflicts, commit to complete the merge.

```bash
twig resolve ours   assets/model.bin
twig resolve theirs assets/model.bin
```

After resolving all conflicts, run `twig commit -m "..."` to complete the merge.

---

### Plumbing Commands

```bash
# Hash and store a file; print its object hash
twig hash-object <file>

# Decode and print a stored object by hash
twig cat-object <hash>
```

These are low-level tools for inspecting the object store directly, useful for debugging.

---

## Configuration

`.twig/config` is a plain `key=value` file. Lines starting with `#` are treated as comments.

```ini
# .twig/config
user.id = alice
```

| Key | Description |
|---|---|
| `user.id` | Author name recorded in commits. Falls back to the OS username if absent. |

---

## Merge Strategy

Twig performs a **three-way merge against the nearest common ancestor**. Non-conflicting changes (modified on only one side) are applied automatically. A conflict is raised when the same path is modified on both sides relative to the ancestor, or when both sides independently add the same path with different content.

Twig does **not** attempt text-level line merging. Every file is treated as an opaque byte sequence. This is a deliberate choice for binary-asset workflows; text files with line-level divergence must be resolved manually.

---

## Instrumentation

Set `TWIG_METRICS=1` before any command to emit a JSON snapshot of internal operation counters to stderr on exit. Useful for profiling deduplication efficiency and diagnosing unexpected rehash activity.

```bash
TWIG_METRICS=1 twig status
```

```json
TWIG_METRICS_DUMP:{"chunker_invocations":0,"hash_file_calls":0,"store_put_calls":12,"store_put_dedup_skips":5}
```

| Counter | Tracks |
|---|---|
| `store_put_calls` | Total calls to `store.Put` |
| `store_put_dedup_skips` | Writes skipped because the object already exists |
| `chunker_invocations` | Number of times the FastCDC chunker was invoked |
| `hash_file_calls` | Number of full file hash computations (triggered by `status`) |

There is no runtime overhead when `TWIG_METRICS` is unset.

---

## Design Tradeoffs

**Loose-only object storage.** Every object is stored as its own individual compressed file. There is no pack-file layer. This keeps the implementation straightforward but is adequate only up to a few thousand objects; very large repositories will encounter filesystem performance limits.

**No text-merge.** All files are treated as opaque byte sequences. This is appropriate for binary-asset workflows but means Twig cannot auto-merge text files with line-level divergence.

---

## Known Limitations & Future Work

- **Garbage collection** — orphaned loose objects accumulate indefinitely.
- **Pack-file layer** — loose-only storage will hit filesystem limits in very large repositories.
- **Remote protocol** — no push/pull; single-machine only.
- **No `.twigignore`** — `twig add .` stages all regular files without exclusion rules.
- **No file mode tracking** — execute bits and symlinks are not preserved in the object model.
- **Text merge** — conflicts on text files require manual resolution; no diff3-style auto-merge.

---

## Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/zeebo/blake3` | v0.2.4 | BLAKE3 content hashing |
| `github.com/jotfs/fastcdc-go` | v0.2.0 | Content-defined chunking (FastCDC) |
| `github.com/fxamacker/cbor/v2` | v2.9.2 | Canonical CBOR serialization |
| `github.com/klauspost/compress` | v1.19.0 | zstd compression |
| `github.com/google/uuid` | v1.6.0 | Used in benchmark harness |
| `modernc.org/sqlite` | v1.53.0 | Used in benchmark harness (SQLite dataset) |

All dependencies are pinned in `go.sum`. The core VCS binary (`cmd/twig`) depends only on the first four; the benchmark binary (`cmd/bench`) uses the full dependency set.

---

## License

MIT — see [LICENSE](LICENSE).
