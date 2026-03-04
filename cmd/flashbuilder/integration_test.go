// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
)

// TestIntegration_DiscoverToGet tests discovery to get flow.
func TestIntegration_DiscoverToGet(t *testing.T) {
	t.Parallel()

	input := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
		"style.css":  &fstest.MapFile{Data: []byte("body {}")},
		"script.js":  &fstest.MapFile{Data: []byte("console.log")},
		"image.png":  &fstest.MapFile{Data: []byte("\x89PNG")},
		"data.json":  &fstest.MapFile{Data: []byte("{}")},
	}

	csp := ""
	// Discover assets
	assets, err := discover(input, csp)
	if err != nil {
		t.Fatalf("discover expected success, got err=%d", err)
	}

	// Set .Identifier
	setIdentifiers(assets)

	assets = deduplicate(assets)

	// Generate variants
	cli := flags{
		Input:    t.TempDir(),
		Output:   t.TempDir(),
		CacheDir: t.TempDir(),
		CacheMax: 99_000_000,
		Brotli:   5,
		AVIF:     50,
		WebP:     50,
	}
	err = copyAssetsAndVariants(input, assets, &cli)
	if err != nil {
		t.Fatalf("copyAssetsAndVariants expected success, got err=%d", err)
	}

	// Allocate embed budget
	const embedBudget = 30
	assets = allocateBudget(assets, embedBudget)

	// Set frequency scores
	for i := range assets {
		assets[i].Frequency = estimateFrequencyScore(assets[i].Path, assets[i].IsEmbedEligible)
	}

	// Add shortcuts
	assets = addShortcutPaths(assets)

	// Compute MaxLen
	maxLen := computeMaxLen(assets)

	// Generate get and post arrays
	get := buildGet(assets, maxLen)
	post := buildPost(assets, maxLen)

	// Verify get array
	if len(get) != maxLen+2 {
		t.Errorf("HTTP get length: expected %d, got %d", maxLen+2, len(get))
	}

	// Verify post array
	if len(post) != 0 {
		t.Errorf("HTTP post length: expected %d, got %d", 0, len(post))
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
		{Path: "a.txt", Size: 100},
		{Path: "b.txt", Size: 200},
		{Path: "c.txt", Size: 300},
	}

	// Budget of 250 should fit a + b, but not c
	result := allocateBudget(assets, 350)

	// Verify first two are eligible
	total := 0
	for _, a := range result {
		if a.IsEmbedEligible {
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
		{Path: "index.html", Identifier: "AssetIndex"},
		{Path: "about/index.html", Identifier: "AssetAboutIndex"},
		{Path: "style.css", Identifier: "AssetStyle"},
	}

	// Build path maps
	canonicalPaths := make(map[string]string)
	shortcutPaths := make(map[string]string)

	for _, asset := range assets {
		if !asset.IsDuplicate {
			canonicalPaths[asset.Path] = asset.Identifier
			shortcut := generateShortcut(asset.Path)
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
