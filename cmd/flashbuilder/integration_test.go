// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestIntegration_DiscoverToGet tests discovery to get flow.
func TestIntegration_DiscoverToGet(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Create test files
	files := []struct {
		name    string
		content string
	}{
		{"index.html", "<html></html>"},
		{"style.css", "body {}"},
		{"script.js", "console.log"},
		{"scrip2.js", "console.log"},
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		err := os.WriteFile(path, []byte(f.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	cli := cli{
		Input:  tmpDir,
		Output: "/tmp/output",
		DryRun: true,
	}

	// Discover assets
	assets, err := discover(cli.Input, cli.CSP)
	if err != nil {
		t.Fatal(err)
	}

	// Set .Identifier and .Filename
	assets = setIdentifiers(assets)

	assets, err = computeHashesETags(assets)
	if err != nil {
		t.Fatal(err)
	}

	assets = deduplicate(assets)

	// Allocate embed budget
	assets = allocateBudget(assets, int64(cli.EmbedBudget))

	// Set frequency scores
	for i := range assets {
		assets[i].Frequency = estimateFrequencyScore(assets[i].RelPath, assets[i].EmbedEligible)
	}

	// Add shortcuts
	assets = addShortcutPaths(assets)

	// Compute MaxLen
	maxLen := computeMaxLen(assets)

	// Generate get arrays
	get := buildGet(assets, maxLen)

	// Verify get arrays
	if len(get) != maxLen+2 {
		t.Errorf("HTTP get length: expected %d, got %d", maxLen+2, len(get))
	}

	want := []handlers{{
		PrevEntry: "",
		Entry:     "",
		Routes:    []asset{},
		Length:    0,
	}, {
		PrevEntry: "",
		Entry:     "",
		Routes:    []asset{},
		Length:    0,
	}, {
		PrevEntry: "",
		Entry:     "",
		Routes:    []asset{},
		Length:    0,
	}, {
		PrevEntry: "",
		Entry:     "",
		Routes:    []asset{},
		Length:    0,
	}}

	if cmp.Equal(get, want) {
		t.Errorf("HTTP get: %v", cmp.Diff(want, get))
	}
}

// TestIntegration_BudgetAllocation tests budget allocation flow.
func TestIntegration_BudgetAllocation(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{RelPath: "a.txt", Size: 100},
		{RelPath: "b.txt", Size: 200},
		{RelPath: "c.txt", Size: 300},
	}

	// Budget of 250 should fit a + b, but not c
	result := allocateBudget(assets, 350)

	// Verify first two are eligible
	total := 0
	for _, a := range result {
		if a.EmbedEligible {
			total++
		}
	}

	if total != 2 {
		t.Errorf("Expected 2 eligible assets, got %d", total)
	}
}

// TestIntegration_ShortcutGeneration tests shortcut generation.
func TestIntegration_ShortcutGeneration(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{RelPath: "index.html", Identifier: "AssetIndex"},
		{RelPath: "about/index.html", Identifier: "AssetAboutIndex"},
		{RelPath: "style.css", Identifier: "AssetStyle"},
	}

	// Build path maps
	canonicalPaths := make(map[string]string)
	shortcutPaths := make(map[string]string)

	for _, asset := range assets {
		if !asset.IsDuplicate {
			canonicalPaths[asset.RelPath] = asset.Identifier
			shortcut := generateShortcut(asset.RelPath)
			if shortcut != "" && canonicalPaths[shortcut] == "" {
				shortcutPaths[shortcut] = asset.Identifier
			}
		}
	}

	// Verify shortcuts
	if canonicalPaths["index.html"] != "AssetIndex" {
		t.Errorf("Expected AssetIndex for index.html")
	}

	// about/index.html should have shortcut "about"
	if shortcutPaths["about"] != "AssetAboutIndex" {
		t.Errorf("Expected AssetAboutIndex for 'about' shortcut")
	}

	// style.css should have shortcut "style"
	if shortcutPaths["style"] != "AssetStyle" {
		t.Errorf("Expected AssetStyle for 'style' shortcut")
	}
}
