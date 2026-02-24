// Package: main
// Purpose: Tests for dispatch array generation, routing, path sanitization
// File: dispatch_test.go

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestBuildDispatch tests dispatch array generation
func TestBuildDispatch(t *testing.T) {
	assets := []asset{
		{RelPath: "index.html", Identifier: "AssetIndex", FrequencyScore: 1000, IsDuplicate: false},
		{RelPath: "style.css", Identifier: "AssetStyle", FrequencyScore: 800, IsDuplicate: false},
		{RelPath: "script.js", Identifier: "AssetScript", FrequencyScore: 600, IsDuplicate: false},
	}

	want := []Handlers{
		{Length: 0, DispatchEntry: "s.ServeIndexHtml", Routes: []RouteData{{Path: "", Identifier: "AssetIndex", Frequency: 1000}}},
		{Length: 0, DispatchEntry: "s.ServeIndexHtml", Routes: []RouteData{{Path: "", Identifier: "AssetIndex", Frequency: 1000}}},
		{Length: 1, DispatchEntry: "s.ServeIndexHtml"},
		{Length: 2, DispatchEntry: "s.ServeIndexHtml"},
		{Length: 3, DispatchEntry: "s.ServeIndexHtml"},
		{Length: 4, DispatchEntry: "s.ServeIndexHtml"},
		{
			Length:        5,
			HandlerName:   "handleLen5",
			PrevEntry:     "s.ServeIndexHtml",
			DispatchEntry: "s.handleLen5",
			Routes:        []RouteData{{Path: "style", Identifier: "AssetStyle", Frequency: 800}},
		},
		{
			Length:        6,
			HandlerName:   "handleLen6",
			PrevEntry:     "s.handleLen5",
			DispatchEntry: "s.handleLen6",
			Routes:        []RouteData{{Path: "script", Identifier: "AssetScript", Frequency: 600}},
		},
		{Length: 7, DispatchEntry: "s.handleLen6"},
		{Length: 8, DispatchEntry: "s.handleLen6"},
		{
			Length:        9,
			HandlerName:   "handleLen9",
			PrevEntry:     "s.handleLen6",
			DispatchEntry: "s.handleLen9",
			Routes: []RouteData{
				{Path: "style.css", Identifier: "AssetStyle", Frequency: 800},
				{Path: "script.js", Identifier: "AssetScript", Frequency: 600}},
		},
		{
			Length:        10,
			HandlerName:   "handleLen10",
			PrevEntry:     "s.handleLen9",
			DispatchEntry: "s.handleLen10",
			Routes:        []RouteData{{Path: "index.html", Identifier: "AssetIndex", Frequency: 1000}},
		},
	}

	assets = addShortcuts(assets)
	maxLen := computeMaxLen(assets)
	dispatch := buildDispatch(assets, maxLen)

	if !cmp.Equal(dispatch, want) {
		t.Errorf("Structs differ: %v", cmp.Diff(dispatch, want))
	}
}

// TestComputeMaxLen tests max length calculation
func TestComputeMaxLen(t *testing.T) {
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
