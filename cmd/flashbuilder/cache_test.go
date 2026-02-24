// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestEnsureCacheDir tests cache directory creation.
func TestEnsureCacheDir(t *testing.T) {
	tmpDir := t.TempDir()

	cachePath := filepath.Join(tmpDir, "cache")

	err := ensureCacheDir(cachePath)
	if err != nil {
		t.Fatalf("ensureCacheDir failed: %v", err)
	}

	// Verify directory exists
	stat, err := os.Stat(cachePath)
	if err != nil {
		t.Fatalf("Cache directory does not exist: %v", err)
	}
	if !stat.IsDir() {
		t.Error("Cache path is not a directory")
	}
}

// TestAllocateBudget tests embed budget allocation.
func TestAllocateBudget(t *testing.T) {
	assets := []asset{
		{RelPath: "small.txt", Size: 100},
		{RelPath: "medium.txt", Size: 500},
		{RelPath: "large.txt", Size: 1000},
	}

	// Budget of 600 should fit small + medium, but not large
	result := allocateBudget(assets, 600)

	// Verify sorting (smallest first)
	if len(result) < 3 {
		t.Fatalf("Expected 3 assets, got %d", len(result))
	}

	// First two should be eligible
	if !result[0].EmbedEligible {
		t.Errorf("Asset 0 should be eligible")
	}
	if !result[1].EmbedEligible {
		t.Errorf("Asset 1 should be eligible")
	}

	// Last one should not be eligible
	if result[2].EmbedEligible {
		t.Errorf("Asset 2 should not be eligible (too large)")
	}
}

// TestCleanCache tests cache cleaning.
func TestCleanCache(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files with different ages
	files := []struct {
		name    string
		modTime time.Time
	}{
		{"old.txt", time.Now().Add(-time.Hour)},
		{"new.txt", time.Now()},
	}

	var maxSize int64
	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		err := os.WriteFile(path, []byte(f.name), 0o644)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
		// Set modification time
		err = os.Chtimes(path, f.modTime, f.modTime)
		if err != nil {
			t.Fatalf("Failed to set mod time: %v", err)
		}
		maxSize = max(maxSize, int64(len(f.name)))
	}

	// Clean cache to remove oldest file (simulate small max size)
	err := cleanCache(tmpDir, maxSize) // 0 bytes max = force deletion
	if err != nil {
		t.Fatalf("cleanCache failed: %v", err)
	}

	// Old file should be deleted
	if _, err := os.Stat(filepath.Join(tmpDir, "old.txt")); err == nil {
		t.Error("Old file should have been deleted")
	}

	// New file should still exist
	if _, err := os.Stat(filepath.Join(tmpDir, "new.txt")); err != nil {
		t.Error("New file should still exist")
	}
}
