package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"twig/internal/index"
	"twig/internal/objects"
	"twig/internal/refs"
	"twig/internal/store"
)

func TestFindCommonAncestor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-ancestor-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// Build a commit graph:
	// C_orig (Origin)
	//   ├── C_A1 ── C_A2 ── C_merge (parents: C_A2, C_B2) ── C_A3
	//   └── C_B1 ── C_B2 ────────────────────────────────── C_B3

	cOrig, err := BuildCommit(s, "tree-orig", nil, "test", "Origin commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	cA1, err := BuildCommit(s, "tree-a1", []string{cOrig}, "test", "A1 commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	cA2, err := BuildCommit(s, "tree-a2", []string{cA1}, "test", "A2 commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	cB1, err := BuildCommit(s, "tree-b1", []string{cOrig}, "test", "B1 commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	cB2, err := BuildCommit(s, "tree-b2", []string{cB1}, "test", "B2 commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	// 1. Basic check: Common ancestor of C_A2 and C_B2 should be C_orig
	ancestor, err := FindCommonAncestor(s, cA2, cB2)
	if err != nil {
		t.Fatalf("FindCommonAncestor failed: %v", err)
	}
	if ancestor != cOrig {
		t.Errorf("expected common ancestor to be %s (origin), got %s", cOrig, ancestor)
	}

	// 2. Identity check: Common ancestor of C_A2 and C_A2 should be C_A2 itself
	ancestor, err = FindCommonAncestor(s, cA2, cA2)
	if err != nil {
		t.Fatalf("FindCommonAncestor failed: %v", err)
	}
	if ancestor != cA2 {
		t.Errorf("expected common ancestor of commit with itself to be %s, got %s", cA2, ancestor)
	}

	// 3. Multi-parent check: Create a merge commit C_merge in branch A with parents [C_A2, C_B2]
	cMerge, err := BuildCommit(s, "tree-merge", []string{cA2, cB2}, "test", "Merge commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	cA3, err := BuildCommit(s, "tree-a3", []string{cMerge}, "test", "A3 commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	cB3, err := BuildCommit(s, "tree-b3", []string{cB2}, "test", "B3 commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	// Common ancestor of C_A3 and C_B3 should be C_B2
	// (because C_A3 -> C_merge -> C_B2, and C_B3 -> C_B2)
	ancestor, err = FindCommonAncestor(s, cA3, cB3)
	if err != nil {
		t.Fatalf("FindCommonAncestor failed: %v", err)
	}
	if ancestor != cB2 {
		t.Errorf("expected common ancestor to be %s (cB2), got %s", cB2, ancestor)
	}

	// 4. Commits sharing no history
	cIsolated, err := BuildCommit(s, "tree-isolated", nil, "test", "Isolated commit")
	if err != nil {
		t.Fatalf("BuildCommit failed: %v", err)
	}

	_, err = FindCommonAncestor(s, cA3, cIsolated)
	if !errors.Is(err, ErrNoCommonAncestor) {
		t.Errorf("expected ErrNoCommonAncestor, got %v", err)
	}
}

func TestDiffTrees(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-difftrees-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s := store.Open(tmpDir)
	if err := s.EnsureLayout(); err != nil {
		t.Fatalf("EnsureLayout failed: %v", err)
	}

	// 1. Build old tree: a.txt, b.txt, c.txt
	oldEntries := map[string]index.Entry{
		"a.txt": {Hash: "hash-a", Type: objects.TypeBlob, Size: 10, ModTime: 100},
		"b.txt": {Hash: "hash-b", Type: objects.TypeBlob, Size: 20, ModTime: 200},
		"c.txt": {Hash: "hash-c", Type: objects.TypeBlob, Size: 30, ModTime: 300},
	}
	oldTreeHash, err := BuildTree(s, oldEntries)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	// 2. Build new tree: a.txt (changed), c.txt (unchanged), d.txt (added)
	newEntries := map[string]index.Entry{
		"a.txt": {Hash: "hash-a-modified", Type: objects.TypeBlob, Size: 15, ModTime: 150},
		"c.txt": {Hash: "hash-c", Type: objects.TypeBlob, Size: 30, ModTime: 300},
		"d.txt": {Hash: "hash-d", Type: objects.TypeBlob, Size: 40, ModTime: 400},
	}
	newTreeHash, err := BuildTree(s, newEntries)
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	// Test 1: includeUnchanged = false
	diffs, err := DiffTrees(s, oldTreeHash, newTreeHash, false)
	if err != nil {
		t.Fatalf("DiffTrees failed: %v", err)
	}

	// Expected diffs:
	// a.txt: DiffChanged
	// b.txt: DiffRemoved
	// d.txt: DiffAdded
	expectedCount := 3
	if len(diffs) != expectedCount {
		t.Fatalf("expected %d diff entries, got %d: %+v", expectedCount, len(diffs), diffs)
	}

	diffMap := make(map[string]DiffEntry)
	for _, d := range diffs {
		diffMap[d.Path] = d
	}

	if entry, ok := diffMap["a.txt"]; !ok || entry.Status != DiffChanged || entry.Hash != "hash-a-modified" {
		t.Errorf("unexpected diff for a.txt: %+v", entry)
	}
	if entry, ok := diffMap["b.txt"]; !ok || entry.Status != DiffRemoved || entry.Hash != "" {
		t.Errorf("unexpected diff for b.txt: %+v", entry)
	}
	if entry, ok := diffMap["d.txt"]; !ok || entry.Status != DiffAdded || entry.Hash != "hash-d" {
		t.Errorf("unexpected diff for d.txt: %+v", entry)
	}
	if _, ok := diffMap["c.txt"]; ok {
		t.Error("expected c.txt to be omitted when includeUnchanged is false")
	}

	// Test 2: includeUnchanged = true
	diffsAll, err := DiffTrees(s, oldTreeHash, newTreeHash, true)
	if err != nil {
		t.Fatalf("DiffTrees failed: %v", err)
	}

	if len(diffsAll) != 4 {
		t.Fatalf("expected 4 diff entries, got %d: %+v", len(diffsAll), diffsAll)
	}

	diffAllMap := make(map[string]DiffEntry)
	for _, d := range diffsAll {
		diffAllMap[d.Path] = d
	}

	if entry, ok := diffAllMap["c.txt"]; !ok || entry.Status != DiffUnchanged || entry.Hash != "hash-c" {
		t.Errorf("expected c.txt to be DiffUnchanged, got %+v", entry)
	}
}

func TestMergeClean(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-merge-clean-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Set author config
	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// 1. Create a.txt and commit on main
	aPath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(aPath, []byte("a content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 2. Create branches feat-1 and feat-2
	if err := r.CreateBranch("feat-1"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := r.CreateBranch("feat-2"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// 3. Checkout feat-1, create b.txt, and commit
	if err := refs.WriteHEAD(r.TwigDir, "feat-1"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	// We also need to reload staging area to match the HEAD (actually, it is already clean)
	bPath := filepath.Join(tmpDir, "b.txt")
	if err := os.WriteFile(bPath, []byte("b content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(bPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	cFeat1, err := r.Commit("Feat 1 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 4. Checkout feat-2, create c.txt, and commit
	if err := refs.WriteHEAD(r.TwigDir, "feat-2"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	// Clean staging index by resetting it to C1
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	os.Remove(indexPath)
	os.Remove(bPath) // remove b.txt from feat-2 working dir

	cPath := filepath.Join(tmpDir, "c.txt")
	if err := os.WriteFile(cPath, []byte("c content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(cPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	cFeat2, err := r.Commit("Feat 2 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 5. Merge feat-1 into feat-2
	if err := r.Merge("feat-1"); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Verify that b.txt has been auto-reconstructed in working dir
	if _, err := os.Stat(bPath); err != nil {
		t.Errorf("expected b.txt to exist: %v", err)
	}
	if _, err := os.Stat(cPath); err != nil {
		t.Errorf("expected c.txt to exist: %v", err)
	}

	// Verify merge commit was created with parents [cFeat2, cFeat1]
	headHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		t.Fatalf("ResolveHEAD failed: %v", err)
	}

	commitBytes, err := r.Store.Get(headHash)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	var commit objects.Commit
	if err := objects.Decode(commitBytes, &commit); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if len(commit.Parents) != 2 || commit.Parents[0] != cFeat2 || commit.Parents[1] != cFeat1 {
		t.Errorf("unexpected merge commit parents: %v", commit.Parents)
	}
}

func TestMergeConflict(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-merge-conflict-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	aPath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(aPath, []byte("origin content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := r.CreateBranch("feat-1"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := r.CreateBranch("feat-2"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// 1. Modify a.txt on feat-1
	if err := refs.WriteHEAD(r.TwigDir, "feat-1"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	if err := os.WriteFile(aPath, []byte("ours content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Feat 1 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 2. Modify a.txt on feat-2
	if err := refs.WriteHEAD(r.TwigDir, "feat-2"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	os.Remove(indexPath)

	if err := os.WriteFile(aPath, []byte("theirs content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Feat 2 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 3. Merge feat-1 into feat-2 -> should result in conflict
	err = r.Merge("feat-1")
	if !errors.Is(err, ErrMergeConflicts) {
		t.Fatalf("expected ErrMergeConflicts, got: %v", err)
	}

	// Staging index should contain conflict markers
	idx, err := index.Load(indexPath)
	if err != nil {
		t.Fatalf("Load index failed: %v", err)
	}

	entry, ok := idx.Get("a.txt")
	if !ok {
		t.Fatalf("a.txt missing from index")
	}

	if entry.Conflict == nil {
		t.Fatalf("expected conflict on a.txt, got nil")
	}

	// WD file should remain "theirs content" (since we checked out feat-2 which was "theirs content")
	wdBytes, err := os.ReadFile(aPath)
	if err != nil {
		t.Fatalf("failed to read a.txt: %v", err)
	}
	if string(wdBytes) != "theirs content" {
		t.Errorf("expected working copy to be 'theirs content', got %q", string(wdBytes))
	}
}

func TestMergeIdentical(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-merge-identical-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	aPath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(aPath, []byte("origin content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := r.CreateBranch("feat-1"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := r.CreateBranch("feat-2"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// 1. Modify a.txt on feat-1 to "same"
	if err := refs.WriteHEAD(r.TwigDir, "feat-1"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	if err := os.WriteFile(aPath, []byte("same"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Feat 1 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 2. Modify a.txt on feat-2 to "same"
	if err := refs.WriteHEAD(r.TwigDir, "feat-2"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	os.Remove(indexPath)

	if err := os.WriteFile(aPath, []byte("same"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Feat 2 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 3. Merge feat-1 into feat-2 -> should merge cleanly since changes are identical
	if err := r.Merge("feat-1"); err != nil {
		t.Fatalf("expected clean merge, got: %v", err)
	}
}

func TestMergeDeleted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-merge-deleted-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	aPath := filepath.Join(tmpDir, "a.txt")
	if err := os.WriteFile(aPath, []byte("origin content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if err := r.CreateBranch("feat-1"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := r.CreateBranch("feat-2"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// 1. Delete a.txt on feat-1
	if err := refs.WriteHEAD(r.TwigDir, "feat-1"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	os.Remove(aPath)
	// Staging index should record removal
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	idx, err := index.Load(indexPath)
	if err != nil {
		t.Fatalf("failed to load index: %v", err)
	}
	idx.Remove("a.txt")
	if err := idx.Save(indexPath); err != nil {
		t.Fatalf("failed to save index: %v", err)
	}
	_, err = r.Commit("Feat 1 commit (deleted a.txt)")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 2. Add c.txt on feat-2
	if err := refs.WriteHEAD(r.TwigDir, "feat-2"); err != nil {
		t.Fatalf("WriteHEAD failed: %v", err)
	}
	os.Remove(indexPath)
	// Write back a.txt (since feat-2 checked out has it)
	if err := os.WriteFile(aPath, []byte("origin content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// We need to stage a.txt in our new index
	if err := r.AddFile(aPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}

	cPath := filepath.Join(tmpDir, "c.txt")
	if err := os.WriteFile(cPath, []byte("c content"), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := r.AddFile(cPath); err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
	_, err = r.Commit("Feat 2 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 3. Merge feat-1 into feat-2 -> should delete a.txt
	if err := r.Merge("feat-1"); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Verify a.txt is removed from WD
	if _, err := os.Stat(aPath); !os.IsNotExist(err) {
		t.Errorf("expected a.txt to be deleted from WD, got err: %v", err)
	}

	// Verify a.txt is removed from index
	idxMerged, err := index.Load(indexPath)
	if err != nil {
		t.Fatalf("Load index failed: %v", err)
	}
	if _, ok := idxMerged.Get("a.txt"); ok {
		t.Error("expected a.txt to be removed from index")
	}
}

func TestResolveConflict(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-resolve-conflict-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(oldCwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}

	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create files a.txt, b.txt, c.txt
	aPath := filepath.Join(tmpDir, "a.txt")
	bPath := filepath.Join(tmpDir, "b.txt")
	cPath := filepath.Join(tmpDir, "c.txt")

	os.WriteFile(aPath, []byte("orig a"), 0644)
	os.WriteFile(bPath, []byte("orig b"), 0644)
	os.WriteFile(cPath, []byte("orig c"), 0644)

	r.AddFile(aPath)
	r.AddFile(bPath)
	r.AddFile(cPath)

	_, err = r.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	r.CreateBranch("feat-1")
	r.CreateBranch("feat-2")

	// 1. Commit modifications on feat-1
	refs.WriteHEAD(r.TwigDir, "feat-1")
	os.WriteFile(aPath, []byte("ours a"), 0644)
	os.WriteFile(bPath, []byte("ours b"), 0644)
	os.Remove(cPath) // ours deletes c.txt

	r.AddFile(aPath)
	r.AddFile(bPath)
	// Stage deletion of c.txt
	indexPath := filepath.Join(r.TwigDir, objects.IndexFileName)
	idx, _ := index.Load(indexPath)
	idx.Remove("c.txt")
	idx.Save(indexPath)

	_, err = r.Commit("Feat 1 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 2. Commit modifications on feat-2
	refs.WriteHEAD(r.TwigDir, "feat-2")
	os.Remove(indexPath)

	// Recreate files with their feat-2 versions
	os.WriteFile(aPath, []byte("theirs a"), 0644)
	os.WriteFile(bPath, []byte("theirs b"), 0644)
	os.WriteFile(cPath, []byte("theirs c"), 0644) // theirs modified c.txt

	r.AddFile(aPath)
	r.AddFile(bPath)
	r.AddFile(cPath)

	_, err = r.Commit("Feat 2 commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 3. Merge feat-1 into feat-2 -> conflicts on a.txt, b.txt, c.txt
	err = r.Merge("feat-1")
	if !errors.Is(err, ErrMergeConflicts) {
		t.Fatalf("expected ErrMergeConflicts, got: %v", err)
	}

	// Verify that resolving a non-existent conflict fails
	err = r.ResolveConflict("nonexistent.txt", "ours")
	if !errors.Is(err, ErrNoConflict) {
		t.Errorf("expected ErrNoConflict, got %v", err)
	}

	// Verify invalid side fails
	err = r.ResolveConflict("a.txt", "invalid-side")
	if err == nil {
		t.Error("expected error for invalid side")
	}

	// 4. Resolve a.txt to ours (which was "ours a")
	// WD copy should remain as "theirs a" (since ours preserves WD)
	// Staging index conflict should be cleared, hash should be the ours version's hash (the hash of "ours a")
	err = r.ResolveConflict("a.txt", "ours")
	if err != nil {
		t.Fatalf("ResolveConflict failed for a.txt: %v", err)
	}

	idx, _ = index.Load(indexPath)
	entryA, ok := idx.Get("a.txt")
	if !ok || entryA.Conflict != nil {
		t.Errorf("a.txt conflict not resolved properly in index: %+v", entryA)
	}

	wdA, _ := os.ReadFile(aPath)
	if string(wdA) != "theirs a" {
		t.Errorf("expected working copy of a.txt to remain 'theirs a', got %q", string(wdA))
	}

	// 5. Resolve b.txt to theirs (which was "theirs b")
	err = r.ResolveConflict("b.txt", "theirs")
	if err != nil {
		t.Fatalf("ResolveConflict failed for b.txt: %v", err)
	}

	idx, _ = index.Load(indexPath)
	entryB, ok := idx.Get("b.txt")
	if !ok || entryB.Conflict != nil {
		t.Errorf("b.txt conflict not resolved properly in index: %+v", entryB)
	}

	// 6. Resolve c.txt to theirs (theirs/feat-1 deleted it)
	// c.txt should be removed from WD and index
	err = r.ResolveConflict("c.txt", "theirs")
	if err != nil {
		t.Fatalf("ResolveConflict failed for c.txt: %v", err)
	}

	if _, err := os.Stat(cPath); !os.IsNotExist(err) {
		t.Error("expected c.txt to be removed from working directory")
	}

	idx, _ = index.Load(indexPath)
	if _, ok := idx.Get("c.txt"); ok {
		t.Error("expected c.txt to be removed from index")
	}
}

func TestMergeIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "twig-merge-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := Init(tmpDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	r, err := Open(tmpDir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	oldCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(oldCwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}

	configContent := "user.id=testuser\n"
	configPath := filepath.Join(r.TwigDir, "config")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// 1. Create initial files on main
	fileAPath := filepath.Join(tmpDir, "fileA.txt")
	fileBPath := filepath.Join(tmpDir, "fileB.txt")
	os.WriteFile(fileAPath, []byte("initial"), 0644)
	os.WriteFile(fileBPath, []byte("initial"), 0644)

	r.AddFile(fileAPath)
	r.AddFile(fileBPath)

	_, err = r.Commit("Initial commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 2. Create feature branch and switch to it
	if err := r.CreateBranch("feature"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := r.Checkout("feature", false); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	// Modify fileA.txt on feature
	os.WriteFile(fileAPath, []byte("feature-mod"), 0644)
	r.AddFile(fileAPath)
	cFeat, err := r.Commit("Feature commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 3. Switch back to main and modify fileB.txt
	if err := r.Checkout("main", false); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	os.WriteFile(fileBPath, []byte("main-mod"), 0644)
	r.AddFile(fileBPath)
	cMain, err := r.Commit("Main commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 4. Merge feature into main (should succeed cleanly)
	if err := r.Merge("feature"); err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	// Verify both file changes are present in working directory
	wdA, _ := os.ReadFile(fileAPath)
	wdB, _ := os.ReadFile(fileBPath)
	if string(wdA) != "feature-mod" || string(wdB) != "main-mod" {
		t.Errorf("expected clean merge content, got fileA=%q, fileB=%q", string(wdA), string(wdB))
	}

	// Verify merge commit parents
	headHash, err := refs.ResolveHEAD(r.TwigDir)
	if err != nil {
		t.Fatalf("ResolveHEAD failed: %v", err)
	}
	commitBytes, err := r.Store.Get(headHash)
	if err != nil {
		t.Fatalf("Get commit failed: %v", err)
	}
	var commit objects.Commit
	if err := objects.Decode(commitBytes, &commit); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(commit.Parents) != 2 || commit.Parents[0] != cMain || commit.Parents[1] != cFeat {
		t.Errorf("unexpected merge commit parents: %v", commit.Parents)
	}

	// 5. Conflicted merge test
	// Create branch-ours and branch-theirs branches from the merge commit
	if err := r.CreateBranch("branch-ours"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := r.CreateBranch("branch-theirs"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Modify fileA.txt on branch-ours
	if err := r.Checkout("branch-ours", false); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}
	os.WriteFile(fileAPath, []byte("ours-content"), 0644)
	r.AddFile(fileAPath)
	cOurs, err := r.Commit("Ours commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Modify fileA.txt on branch-theirs
	if err := r.Checkout("branch-theirs", false); err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}
	os.WriteFile(fileAPath, []byte("theirs-content"), 0644)
	r.AddFile(fileAPath)
	cTheirs, err := r.Commit("Theirs commit")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Merge branch-ours into branch-theirs -> conflicts!
	err = r.Merge("branch-ours")
	if !errors.Is(err, ErrMergeConflicts) {
		t.Fatalf("expected ErrMergeConflicts, got: %v", err)
	}

	// Attempting to commit should fail
	_, err = r.Commit("Try bypass conflicts")
	if !errors.Is(err, ErrMergeConflicts) {
		t.Fatalf("expected commit to be blocked with ErrMergeConflicts, got %v", err)
	}

	// Verify that Status reports StatusConflict
	statuses, err := r.Status()
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	foundConflict := false
	for _, st := range statuses {
		if st.Path == "fileA.txt" && st.Status == StatusConflict {
			foundConflict = true
			break
		}
	}
	if !foundConflict {
		t.Error("expected Status to report StatusConflict for fileA.txt")
	}

	// Resolve in favor of ours (keeps local WD copy which is theirs-content, but stages ours-content)
	err = r.ResolveConflict("fileA.txt", "ours")
	if err != nil {
		t.Fatalf("ResolveConflict failed: %v", err)
	}

	// Commit should succeed now
	cMergeResolved, err := r.Commit("Resolved conflict commit")
	if err != nil {
		t.Fatalf("Commit failed after resolve: %v", err)
	}

	// Verify that the merge resolved commit has both parents
	resolvedBytes, err := r.Store.Get(cMergeResolved)
	if err != nil {
		t.Fatalf("Get commit failed: %v", err)
	}
	var resolvedCommit objects.Commit
	if err := objects.Decode(resolvedBytes, &resolvedCommit); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(resolvedCommit.Parents) != 2 || resolvedCommit.Parents[0] != cTheirs || resolvedCommit.Parents[1] != cOurs {
		t.Errorf("expected resolved commit to have parents [%s, %s], got %v", cTheirs, cOurs, resolvedCommit.Parents)
	}

	// Verify that MERGE_HEAD has been cleaned up
	mergeHeadPath := filepath.Join(r.TwigDir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHeadPath); !os.IsNotExist(err) {
		t.Error("expected MERGE_HEAD file to be removed after successful commit")
	}
}

