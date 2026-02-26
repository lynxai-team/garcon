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
		{RelPath: "index.html", Identifier: "IndexHtml", FrequencyScore: 1000, IsDuplicate: false},
		{RelPath: "style.css", Identifier:  "StyleCss", FrequencyScore: 800, IsDuplicate: false},
		{RelPath: "script.js", Identifier:  "ScriptJs", FrequencyScore: 600, IsDuplicate: false},
	}

	want := []handlers{
		{Length: 0, Entry: "serveIndexHtml", Routes: []routeData{{Path: "", Identifier: "IndexHtml", Frequency: 1000}}},
		{Length: 0, Entry: "serveIndexHtml", Routes: []routeData{{Path: "", Identifier: "IndexHtml", Frequency: 1000}}},
		{Length: 1, Entry: "serveIndexHtml"},
		{Length: 2, Entry: "serveIndexHtml"},
		{Length: 3, Entry: "serveIndexHtml"},
		{Length: 4, Entry: "serveIndexHtml"},
		{
			Length:    5,
			PrevEntry: "serveIndexHtml",
			Entry:     "handleLen5",
			Routes:    []routeData{{Path: "style", Identifier: "StyleCss", Frequency: 800}},
		},
		{
			Length:    6,
			PrevEntry: "handleLen5",
			Entry:     "handleLen6",
			Routes:    []routeData{{Path: "script", Identifier: "ScriptJs", Frequency: 600}},
		},
		{Length: 7, Entry: "handleLen6"},
		{Length: 8, Entry: "handleLen6"},
		{
			Length:    9,
			PrevEntry: "handleLen6",
			Entry:     "handleLen9",
			Routes: []routeData{
				{Path: "style.css", Identifier: "StyleCss", Frequency: 800},
				{Path: "script.js", Identifier: "ScriptJs", Frequency: 600},
			},
		},
		{
			Length:    10,
			PrevEntry: "handleLen9",
			Entry:     "handleLen10",
			Routes:    []routeData{{Path: "index.html", Identifier: "IndexHtml", Frequency: 1000}},
		},
	}

	assets = addShortcuts(assets)
	maxLen := computeMaxLen(assets)
	dispatch := buildDispatch(assets, maxLen)

	if !cmp.Equal(dispatch, want) {
		t.Errorf("Structs differ: %v", cmp.Diff(dispatch, want))
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
