// Package: main
// Purpose: Tests for dispatch array generation, routing, path sanitization
// File: dispatch_test.go

package main

import (
	"testing"
)

// TestBuildDispatch tests dispatch array generation
func TestBuildDispatch(t *testing.T) {
	assets := []asset{
		{RelPath: "index.html", Identifier: "AssetIndex", FrequencyScore: 1000, IsDuplicate: false},
		{RelPath: "style.css", Identifier: "AssetStyle", FrequencyScore: 800, IsDuplicate: false},
		{RelPath: "script.js", Identifier: "AssetScript", FrequencyScore: 600, IsDuplicate: false},
	}

	maxLen := computeMaxLen(assets)
	httpDispatch, httpsDispatch := buildDispatch(assets, maxLen)

	// Verify dispatch arrays have correct length
	expectedLen := maxLen + 2
	if len(httpDispatch) != expectedLen {
		t.Errorf("HTTP dispatch length: expected %d, got %d", expectedLen, len(httpDispatch))
	}
	if len(httpsDispatch) != expectedLen {
		t.Errorf("HTTPS dispatch length: expected %d, got %d", expectedLen, len(httpsDispatch))
	}

	// Verify root handler at index 0 and 1
	if httpDispatch[0].Handler != "serveRootIndex" {
		t.Errorf("HTTP dispatch[0]: expected serveRootIndex, got %s", httpDispatch[0].Handler)
	}
	if httpDispatch[1].Handler != "serveRootIndex" {
		t.Errorf("HTTP dispatch[1]: expected serveRootIndex, got %s", httpDispatch[1].Handler)
	}

	// Verify routes are sorted by frequency
	for i := 2; i < len(httpDispatch); i++ {
		if len(httpDispatch[i].Routes) > 1 {
			for j := 1; j < len(httpDispatch[i].Routes); j++ {
				if httpDispatch[i].Routes[j-1].Frequency < httpDispatch[i].Routes[j].Frequency {
					t.Errorf("Routes not sorted by frequency at dispatch[%d]", i)
				}
			}
		}
	}
}

// TestSanitizePath tests path sanitization for switch cases
func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple path", "style.css", "style.css"},
		{"Path with backslash", "path\\to\\file", "path/to/file"},
		{"Path with quote", `path"file`, "path\\\"file"},
		{"Path with newline", "path\nfile", "path\\nfile"},
		{"Path with tab", "path\tfile", "path\\tfile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizePath(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
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

// TestHasRootIndex tests root index detection
func TestHasRootIndex(t *testing.T) {
	tests := []struct {
		name     string
		assets   []asset
		expected bool
	}{
		{"Has index.html", []asset{{RelPath: "index.html"}}, true},
		{"Has empty path", []asset{{RelPath: ""}}, true},
		{"No index", []asset{{RelPath: "style.css"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRootIndex(tt.assets)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
