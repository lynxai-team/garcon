// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIntegration_DiscoverToDispatch tests discovery to dispatch flow.
func TestIntegration_DiscoverToDispatch(t *testing.T) {
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
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		err := os.WriteFile(path, []byte(f.content), 0o644)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Run discovery
	assets, err := discover(tmpDir)
	if err != nil {
		t.Fatalf("Discovery failed: %v", err)
	}

	// Verify assets
	if len(assets) != len(files) {
		t.Errorf("Expected %d assets, got %d", len(files), len(assets))
	}

	// Compute hashes
	for i := range assets {
		assets[i].ImoHash = computeImoHash(assets[i].AbsPath)
		assets[i].ETag = computeETag(assets[i].ImoHash)
	}

	// Generate identifiers
	identifiers := make(map[string]bool)
	for i := range assets {
		assets[i].Identifier = generateIdentifier(assets[i].RelPath, identifiers)
		identifiers[assets[i].Identifier] = true
		assets[i].Filename = assets[i].Identifier + filepath.Ext(assets[i].RelPath)
	}

	// Compute frequency scores
	for i := range assets {
		assets[i].FrequencyScore = estimateFrequencyScore(assets[i].RelPath, assets[i].EmbedEligible)
	}

	// Compute max length
	maxLen := computeMaxLen(assets)
	if maxLen <= 0 {
		t.Errorf("MaxLen should be positive, got %d", maxLen)
	}

	// Build dispatch
	dispatch := buildDispatch(assets, maxLen)

	// Verify dispatch arrays
	if len(dispatch) != maxLen+2 {
		t.Errorf("HTTP dispatch length: expected %d, got %d", maxLen+2, len(dispatch))
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
