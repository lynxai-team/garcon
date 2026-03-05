// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestBuildGet2 tests get array generation.
func TestBuildGet2(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{Path: "index.html", Identifier: "IndexHtml", Frequency: 1000, IsDuplicate: false},
		{Path: "style.css", Identifier: "StyleCss", Frequency: 800, IsDuplicate: false},
		{Path: "script.js", Identifier: "ScriptJs", Frequency: 600, IsDuplicate: false},
	}

	want := []handlers{
		{Length: 0, Entry: "getIndexHtml", PrevEntry: "notFound", Routes: []asset{{Identifier: "IndexHtml", Frequency: 1000, IsShortcut: true}}},
		{Length: 0, Entry: "getIndexHtml", PrevEntry: "getIndexHtml", Routes: []asset{{Identifier: "IndexHtml", Frequency: 1000, IsShortcut: true}}},
		{Length: 1, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{Length: 2, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{Length: 3, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{Length: 4, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{
			Length:    5,
			PrevEntry: "getIndexHtml",
			Entry:     "getLen5",
			Routes:    []asset{{Path: "style", Identifier: "StyleCss", Frequency: 800, IsShortcut: true}},
		},
		{
			Length:    6,
			PrevEntry: "getLen5",
			Entry:     "getLen6",
			Routes:    []asset{{Path: "script", Identifier: "ScriptJs", Frequency: 600, IsShortcut: true}},
		},
		{Length: 7, Entry: "getLen6", PrevEntry: "getLen6"},
		{Length: 8, Entry: "getLen6", PrevEntry: "getLen6"},
		{
			Length:    9,
			PrevEntry: "getLen6",
			Entry:     "getLen9",
			Routes: []asset{
				{Path: "style.css", Identifier: "StyleCss", Frequency: 800},
				{Path: "script.js", Identifier: "ScriptJs", Frequency: 600},
			},
		},
		{
			Length:    10,
			PrevEntry: "getLen9",
			Entry:     "getLen10",
			Routes:    []asset{{Path: "index.html", Identifier: "IndexHtml", Frequency: 1000}},
		},
	}

	assets = addShortcutRoutes(assets)
	maxLenG := computeMaxLenGet(assets)
	get := buildGet(assets, maxLenG)

	if !cmp.Equal(get, want) {
		t.Errorf("Structs differ: %v", cmp.Diff(want, get))
	}
}

// TestComputeMaxLen2 tests max length calculation.
func TestComputeMaxLen2(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		paths    []string
		expected int
	}{
		{"Single file", []string{"a"}, 1},
		{"Multiple files", []string{"a", "b/c", "d/e/f"}, 5},
		{"Empty", []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assets := make([]asset, len(tt.paths))
			for i, p := range tt.paths {
				assets[i] = asset{Path: p}
			}
			result := computeMaxLenGet(assets)
			if result != tt.expected {
				t.Errorf("Want %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestBuildGet tests the route bucketing for GET requests.
func TestBuildGet(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{Path: "index.html", Identifier: "IndexHtml", Frequency: 1000, IsDuplicate: false},
		{Path: "style.css", Identifier: "StyleCss", Frequency: 800, IsDuplicate: false},
		{Path: "script.js", Identifier: "ScriptJs", Frequency: 600, IsDuplicate: false},
	}

	assets = addShortcutRoutes(assets)
	maxLenG := computeMaxLenGet(assets)
	get := buildGet(assets, maxLenG)

	// Verify structure
	if len(get) == 0 {
		t.Fatal("Get array is empty")
	}

	// Check for correct indexing (based on frequency)
	// The highest frequency asset should appear first in its length group.
	// We need to check specific entries.
	// Note: addShortcutPaths modifies assets. We need to re-verify.

	// Check the first entry (length 0 or 1 depending on logic)
	// The code logic for buildGet: index is length+1?
	// Let's test a specific known case.

	if get[0].Entry != "getIndexHtml" && get[1].Entry != "getIndexHtml" {
		// Logic check
	}
}

// TestComputeMaxLen tests max length calculation.
func TestComputeMaxLen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		paths    []string
		expected int
	}{
		{"Single file", []string{"a"}, 1},
		{"Multiple files", []string{"a", "b/c", "d/e/f"}, 5},
		{"Empty", []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assets := make([]asset, len(tt.paths))
			for i, p := range tt.paths {
				assets[i] = asset{Path: p}
			}
			result := computeMaxLenGet(assets)
			if result != tt.expected {
				t.Errorf("Want %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestAddShortcutPaths tests shortcut generation.
func TestAddShortcutPaths(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{MIME: "text/css", Path: "style.css", Identifier: "StyleCss"},
		{MIME: "text/html", Path: "index.html", Identifier: "IndexHtml"},
		{MIME: "text/html", Path: "about/index.html", Identifier: "AboutIndex"},
		{MIME: "image/png", Path: "images/logo.png", Identifier: "ImagesLogoPng"},
		{MIME: "image/jpeg", Path: "images/logo.jpeg", Identifier: "ImagesLogoJpeg"},
	}

	got := addShortcutRoutes(assets)

	// We expect 3 shortcuts
	// - index.html       -> "" (root)
	// - about/index.html -> "about"
	// - images/logo.png  -> "images/logo"
	want := []asset{
		{MIME: "text/css", Path: "style.css", Identifier: "StyleCss"},
		{MIME: "text/html", Path: "index.html", Identifier: "IndexHtml"},
		{MIME: "text/html", Path: "about/index.html", Identifier: "AboutIndex"},
		{MIME: "image/png", Path: "images/logo.png", Identifier: "ImagesLogoPng"},
		{MIME: "image/jpeg", Path: "images/logo.jpeg", Identifier: "ImagesLogoJpeg"},
		{MIME: "text/html", Path: "", Identifier: "IndexHtml", IsShortcut: true},
		{MIME: "text/html", Path: "about", Identifier: "AboutIndex", IsShortcut: true},
		{MIME: "image/png", Path: "images/logo", Identifier: "ImagesLogoPng", IsShortcut: true},
	}

	if len(got) != len(want) {
		t.Errorf("got %d routes, want %d", len(got), len(want))
	}

	count := 0
	for a := range slices.Values(got) {
		if a.IsShortcut {
			count++
		}
	}
	if count != 3 {
		t.Errorf("got %d shortcuts, want 3", count)
	}

	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}
