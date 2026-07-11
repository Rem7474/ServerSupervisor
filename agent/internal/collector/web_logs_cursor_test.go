package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if identityChanged(path, info) {
		t.Error("first sight of a path should never report a change")
	}

	info2, err := os.Stat(path) // same underlying file, re-stat
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if identityChanged(path, info2) {
		t.Error("re-stat of the same, unrotated file should not report a change")
	}

	// Simulate a realistic rotation (logrotate-style rename, not delete): the
	// old file is moved aside — keeping its inode alive under the new name —
	// and a genuinely new file is created at the original path. Removing
	// the old file outright instead of renaming it would free its inode
	// immediately, which a filesystem can (and, observed in practice, does)
	// hand straight back to the very next file created in the same
	// directory — making the "new" file indistinguishable by identity from
	// the old one for reasons that have nothing to do with the rotation
	// detection logic under test.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if err := os.WriteFile(path, []byte("hello again\n"), 0o644); err != nil {
		t.Fatalf("WriteFile (recreate): %v", err)
	}
	info3, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !identityChanged(path, info3) {
		t.Error("a recreated file at the same path should report a change")
	}
}

func TestSaveAndLoadWebLogCursor_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "cursor.json")

	state := &webLogCursorState{Files: map[string]webLogCursorEntry{
		"/var/log/nginx/access.log": {Offset: 1234, Size: 1234, FileModUnix: 1700000000},
	}}
	saveWebLogCursor(cursorPath, state)

	loaded := loadWebLogCursor(cursorPath)
	if got := loaded.Files["/var/log/nginx/access.log"].Offset; got != 1234 {
		t.Errorf("Offset = %d, want 1234", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("directory contains %d entries after save, want 1 (no leftover temp file): %v", len(entries), entries)
	}
}

func TestSaveWebLogCursor_AtomicallyReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	cursorPath := filepath.Join(dir, "cursor.json")

	saveWebLogCursor(cursorPath, &webLogCursorState{Files: map[string]webLogCursorEntry{
		"a": {Offset: 1},
	}})
	saveWebLogCursor(cursorPath, &webLogCursorState{Files: map[string]webLogCursorEntry{
		"b": {Offset: 2},
	}})

	loaded := loadWebLogCursor(cursorPath)
	if _, ok := loaded.Files["a"]; ok {
		t.Error("stale entry from the first save survived the second save — not a clean replace")
	}
	if got := loaded.Files["b"].Offset; got != 2 {
		t.Errorf("Offset = %d, want 2", got)
	}
}

func TestLoadWebLogCursor_MissingOrCorruptFileFailsOpen(t *testing.T) {
	dir := t.TempDir()

	missing := loadWebLogCursor(filepath.Join(dir, "does-not-exist.json"))
	if missing == nil || missing.Files == nil {
		t.Error("loadWebLogCursor for a missing file should return a usable empty state, not nil")
	}

	corruptPath := filepath.Join(dir, "corrupt.json")
	if err := os.WriteFile(corruptPath, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	corrupt := loadWebLogCursor(corruptPath)
	if corrupt == nil || corrupt.Files == nil || len(corrupt.Files) != 0 {
		t.Errorf("loadWebLogCursor for a corrupt file should return a usable empty state, got %+v", corrupt)
	}
}

// TestReadIncrementalLines_FastRotationDetectedEvenWhenSizeGrew is the
// regression test for the bug the size-only check missed: a log rotated
// (renamed out, brand new file created at the same path) and, by the time
// the next scan ran, the new file had already grown past the old file's
// recorded offset — so "new size < old offset" never fires even though it's
// a completely different file. Without the file-identity check this would
// seek into the new file at a byte offset unrelated to any line boundary
// and return garbage instead of a clean bootstrap of the new file.
func TestReadIncrementalLines_FastRotationDetectedEvenWhenSizeGrew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotating.log")

	writeLines := func(lines ...string) {
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	// File A: the "old" file this agent already scanned once.
	writeLines("line-a-1", "line-a-2", "line-a-3")
	_, entryA, err := readIncrementalLines(path, 100, webLogCursorEntry{}, false)
	if err != nil {
		t.Fatalf("initial read of file A returned error: %v", err)
	}

	// Rotate the realistic (logrotate-style) way: rename file A aside rather
	// than deleting it, so its inode stays alive and the brand new file B
	// created below at the original path is guaranteed a different identity
	// — not just "probably different because nothing reused the freed inode
	// in this empty test directory". A brand new file B (different content,
	// same path) is bigger than file A ever was — bigger than entryA.Offset.
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	bLines := []string{"line-b-1", "line-b-2", "line-b-3", "line-b-4", "line-b-5", "line-b-6", "line-b-7", "line-b-8", "line-b-9", "line-b-10"}
	writeLines(bLines...)

	infoB, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if infoB.Size() <= entryA.Offset {
		t.Fatalf("test setup invalid: file B (%d bytes) must be larger than file A's recorded offset (%d) to reproduce the bug", infoB.Size(), entryA.Offset)
	}

	got, _, err := readIncrementalLines(path, 100, entryA, true)
	if err != nil {
		t.Fatalf("read after rotation returned error: %v", err)
	}

	if len(got) != len(bLines) {
		t.Fatalf("got %d lines after rotation, want %d (a clean bootstrap of file B): %v", len(got), len(bLines), got)
	}
	for i, want := range bLines {
		if got[i] != want {
			t.Errorf("line %d = %q, want %q — looks like a raw seek into file B rather than a bootstrap", i, got[i], want)
		}
	}
}
