// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestBuildDispatch tests dispatch array generation.
func TestBuildDispatch(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{RelPath: "index.html", Identifier: "IndexHtml", Frequency: 1000, IsDuplicate: false},
		{RelPath: "style.css", Identifier: "StyleCss", Frequency: 800, IsDuplicate: false},
		{RelPath: "script.js", Identifier: "ScriptJs", Frequency: 600, IsDuplicate: false},
	}

	want := []handlers{
		{Length: 0, Entry: "serveIndexHtml", PrevEntry: "notFound", Routes: []asset{{Identifier: "IndexHtml", Frequency: 1000, IsShortcut: true}}},
		{Length: 0, Entry: "serveIndexHtml", PrevEntry: "serveIndexHtml", Routes: []asset{{Identifier: "IndexHtml", Frequency: 1000, IsShortcut: true}}},
		{Length: 1, Entry: "serveIndexHtml", PrevEntry: "serveIndexHtml"},
		{Length: 2, Entry: "serveIndexHtml", PrevEntry: "serveIndexHtml"},
		{Length: 3, Entry: "serveIndexHtml", PrevEntry: "serveIndexHtml"},
		{Length: 4, Entry: "serveIndexHtml", PrevEntry: "serveIndexHtml"},
		{
			Length:    5,
			PrevEntry: "serveIndexHtml",
			Entry:     "handleLen5",
			Routes:    []asset{{RelPath: "style", Identifier: "StyleCss", Frequency: 800, IsShortcut: true}},
		},
		{
			Length:    6,
			PrevEntry: "handleLen5",
			Entry:     "handleLen6",
			Routes:    []asset{{RelPath: "script", Identifier: "ScriptJs", Frequency: 600, IsShortcut: true}},
		},
		{Length: 7, Entry: "handleLen6", PrevEntry: "handleLen6"},
		{Length: 8, Entry: "handleLen6", PrevEntry: "handleLen6"},
		{
			Length:    9,
			PrevEntry: "handleLen6",
			Entry:     "handleLen9",
			Routes: []asset{
				{RelPath: "style.css", Identifier: "StyleCss", Frequency: 800},
				{RelPath: "script.js", Identifier: "ScriptJs", Frequency: 600},
			},
		},
		{
			Length:    10,
			PrevEntry: "handleLen9",
			Entry:     "handleLen10",
			Routes:    []asset{{RelPath: "index.html", Identifier: "IndexHtml", Frequency: 1000}},
		},
	}

	assets = addShortcutPaths(assets)
	maxLen := computeMaxLen(assets)
	dispatch := buildDispatch(assets, maxLen)

	if !cmp.Equal(dispatch, want) {
		t.Errorf("Structs differ: %v", cmp.Diff(want, dispatch))
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
				assets[i] = asset{RelPath: p}
			}
			result := computeMaxLen(assets)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}
