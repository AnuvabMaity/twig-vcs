<p align="center">
  <img src="assets/logo.svg" width="140" alt="Twig Logo">
  <br>
  <strong>Content-addressable version control system for binary-heavy repositories</strong>
  <br><br>
  <a href="https://go.dev/">
    <img src="https://img.shields.io/badge/go-1.21%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go Version">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/badge/license-MIT-brightgreen?style=flat-square" alt="License: MIT">
  </a>
  <a href="#">
    <img src="https://img.shields.io/badge/build-passing-brightgreen?style=flat-square" alt="Build">
  </a>
</p>

---

## What is Twig?

Standard VCS tools handle binary files poorly. Git stores each revision of a binary asset as a separate compressed blob; Git LFS improves storage but requires external infrastructure. Neither exploits the internal structure of evolving binary files.

Twig uses **FastCDC** (content-defined chunking) to split files into variable-size chunks at content-dependent boundaries. When a file changes, only the chunks that actually changed are written — unchanged chunks are deduplicated automatically across all revisions.

This makes Twig a practical fit for repositories that track iteratively-modified binary assets — SQLite databases, game assets, or similar append-heavy workloads. For repositories dominated by small text files or highly compressed data (e.g., JPEGs, MP4s), the deduplication benefit is negligible.

---

## Features

- **Content-Defined Chunking** — files ≥ 16 KB are split by FastCDC; only changed chunks are stored per revision.
- **Automatic deduplication** — identical chunks across any files or commits are stored exactly once.
- **Git-like workflow** — `init`, `add`, `commit`, `log`, `status`, `checkout`, `branch`, `merge`.
- **Three-way merge** — merges against the nearest common ancestor; non-conflicting changes apply automatically.
- **Conflict resolution** — conflicting files are flagged with `ours`/`theirs` sides; resolved with `twig resolve`.
- **mtime-aware status** — `status` skips re-hashing files whose size and modification time are unchanged.
- **Single binary** — no runtime dependencies; builds to a single self-contained executable.
- **BLAKE3 content addressing** — every stored object is identified by its BLAKE3 hash; integrity is always verifiable.
- **zstd compression** — all objects are transparently compressed at rest.

---

## Architecture Overview

```
Working Directory
       │
       │  twig add
       ▼
  Staging Index
       │
       │  twig commit
       ▼
  Tree Objects  ──►  Commit Objects
       │                    │
       └──────┬─────────────┘
              ▼
    Loose Object Store
    .twig/objects/<hash[:2]>/<hash[2:]>
              │
    ┌─────────┴──────────┐
    │                    │
  Blob               Asset + Chunks
  (< 16 KB)          (≥ 16 KB, FastCDC)
```

Files under 16 KB are stored whole. Larger files are split by FastCDC into variable-size chunks; only a manifest of chunk references is stored at the top level. All objects are content-addressed and share the same store.

---

## On-Disk Layout

```
.twig/
├── HEAD           # current branch or detached commit
├── VERSION        # storage format version
├── config         # repository configuration
├── index          # staging area
├── MERGE_HEAD     # present only during an active merge
├── objects/
│   └── <xx>/      # 2-char fan-out prefix
│       └── <yy…>  # object file (zstd-compressed)
└── refs/
    └── heads/
        └── <branch-name>
```

---

## Building & Running

**Prerequisites:** Go 1.21 or later.

```bash
# Clone and build
git clone https://github.com/AnuvabMaity/twig
cd twig
go build -o twig ./cmd/twig

# Run tests
go test ./...
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

| Status | Meaning |
|---|---|
| `staged-new` | Staged; not in the last commit |
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

With no arguments, lists all branches with `*` marking the current one. With a name, creates a new branch at the current commit.

```bash
twig branch              # list branches
twig branch feature-x    # create branch at HEAD
```

---

### `twig merge <branch>`

Merges the named branch into the current branch. Non-conflicting changes are applied automatically; conflicting files are flagged and require manual resolution.

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

Twig performs a **three-way merge against the nearest common ancestor**. Non-conflicting changes — modified on only one side — are applied automatically. A conflict is raised when the same path is modified on both sides, or when both sides independently add the same path with different content.

Twig does **not** attempt text-level line merging. Every file is treated as an opaque byte sequence. Text files with line-level divergence must be resolved manually with `twig resolve`.

---

## Instrumentation

Set `TWIG_METRICS=1` to emit a JSON counter snapshot to stderr on exit. Useful for profiling deduplication efficiency.

```bash
TWIG_METRICS=1 twig add .
```

```json
TWIG_METRICS_DUMP:{"chunker_invocations":4,"hash_file_calls":0,"store_put_calls":38,"store_put_dedup_skips":12}
```

---

## Known Limitations & Future Work

- **No garbage collection** — unreferenced objects accumulate and are never reclaimed.
- **Local-only** — no push/pull/clone; single-machine only.
- **No `.twigignore`** — `twig add .` stages all files without exclusion rules.
- **No file mode tracking** — execute bits and symlinks are not preserved.
- **No text merge** — line-level auto-merge is not supported.
- **Loose-only storage** — no pack-file consolidation; very large repositories will hit filesystem limits.

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
