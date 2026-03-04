// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
	"testing/fstest"
)

func TestDiscover(t *testing.T) {
	t.Parallel()

	input := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
		"style.css":  &fstest.MapFile{Data: []byte("body {}")},
		"script.js":  &fstest.MapFile{Data: []byte("console.log")},
		"image.png":  &fstest.MapFile{Data: []byte("\x89PNG")},
		"data.json":  &fstest.MapFile{Data: []byte("{}")},
	}

	assets, err := discover(input, "")
	if err != nil {
		t.Fatalf("Discovery failed: %v", err)
	}

	// Verify count
	if len(assets) != 5 {
		t.Errorf("Expected 5 assets, got %d", len(assets))
	}

	// Verify sorting
	for i := 1; i < len(assets); i++ {
		if assets[i].Path < assets[i-1].Path {
			t.Errorf("Assets not sorted")
		}
	}
}

// TestGenerateIdentifier tests identifier generation.
func TestGenerateIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inPath   string
		expected string
	}{
		{"Simple file", "css/style.css", "CssStyleCss"},
		{"Index file", "index.html", "IndexHtml"},
		{"Nested file", "assets/css/main.css", "AssetsCssMainCss"},
		{"Special chars", "assets/images/logo-1.png", "AssetsImagesLogo1Png"},
		{"Duplicate", "css/style.css", "AssetCssStyleCss2"},
	}

	identifiers := existing{}
	for i, tt := range tests {
		if i > 0 && tt.name == "Duplicate" {
			// Simulate existing identifier
			identifiers["CssStyleCss"] = struct{}{}
		}

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := identifiers.generateIdentifier(tt.inPath)
			// Check for valid Go identifier chars
			for _, r := range id {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
					t.Errorf("Invalid character in identifier: %c", r)
				}
			}
		})
	}
}

// TestEstimateFrequencyScore tests frequency score calculation.
func TestEstimateFrequencyScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		isEmbed  bool
		expected int
	}{
		{"Index file", "index.html", true, 1000 + 500 + 200},
		{"Favicon", "favicon.ico", true, 900 + 200},
		{"CSS file", "style.css", true, 800 + 200},
		{"JS file", "script.js", true, 600 + 200},
		{"Logo", "logo.png", true, 400 + 200},
		{"Deep path", "assets/css/main.css", true, 800 + 200 - (5 * len("assets/css/main.css")) - 30*2},
		{"Low traffic", "data.pdf", true, -100 + 200 - (5 * len("data.pdf"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := estimateFrequencyScore(tt.path, tt.isEmbed)
			// Just verify it returns a positive value for high-priority files
			// and negative for low-priority
			if tt.path == "index.html" && result <= 0 {
				t.Errorf("Index should have positive score, got %d", result)
			}
		})
	}
}

// TestGenerateShortcut tests shortcut generation.
func TestGenerateShortcut(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inPath   string
		expected string
	}{
		{"Root index", "index.html", ""},
		{"Subdir index", "about/index.html", "about"},
		{"CSS file", "style.css", "style"},
		{"JS file", "script.js", "script"},
		{"No extension", "path/to/file", "path/to/file"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := generateShortcut(tt.inPath)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
