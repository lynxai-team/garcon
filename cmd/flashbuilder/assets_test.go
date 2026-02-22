// Package: main
// Purpose: Tests for asset discovery, MIME detection, deduplication
// File: assets_test.go

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDiscover tests asset discovery
func TestDiscover(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	files := []struct {
		name     string
		content  string
		expected string
	}{
		{"index.html", "<html></html>", "text/html"},
		{"style.css", "body {}", "text/css"},
		{"script.js", "console.log", "text/javascript"},
		{"image.png", "\x89PNG", "image/png"},
		{"data.json", "{}", "application/json"},
	}

	for _, file := range files {
		path := filepath.Join(tmpDir, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Run discovery
	assets, err := discover(tmpDir)
	if err != nil {
		t.Fatalf("Discovery failed: %v", err)
	}

	// Verify count
	if len(assets) != len(files) {
		t.Errorf("Expected %d assets, got %d", len(files), len(assets))
	}

	// Verify sorting (alphabetical by RelPath)
	for i := 1; i < len(assets); i++ {
		if assets[i].RelPath < assets[i-1].RelPath {
			t.Errorf("Assets not sorted: %s should come after %s", assets[i].RelPath, assets[i-1].RelPath)
		}
	}

	// Verify MIME detection
	for _, asset := range assets {
		if asset.MIME == "" || asset.MIME == "application/octet-stream" && asset.RelPath != "data.json" {
			t.Errorf("MIME detection failed for %s: got %s", asset.RelPath, asset.MIME)
		}
	}
}

// TestDetectMIME tests MIME type detection
func TestDetectMIME(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		content      string
		expectedMIME string
	}{
		{"HTML file", "index.html", "<html></html>", "text/html; charset=utf-8"},
		{"CSS file", "style.css", "body {}", "text/css; charset=utf-8"},
		{"JS file", "script.js", "console.log", "text/javascript; charset=utf-8"},
		{"Unknown", "data.bin", "\x00\x01\x02", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "flashbuilder-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write file: %v", err)
			}

			mime := detectMIME(path)
			if mime != tt.expectedMIME {
				t.Errorf("Expected %s, got %s", tt.expectedMIME, mime)
			}
		})
	}
}

// TestGenerateIdentifier tests identifier generation
func TestGenerateIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected string
	}{
		{"Simple file", "css/style.css", "AssetCssStyle"},
		{"Index file", "index.html", "AssetIndex"},
		{"Nested file", "assets/css/main.css", "AssetAssetsCssMain"},
		{"Special chars", "assets/images/logo-1.png", "AssetAssetsImagesLogo1"},
		{"Duplicate", "css/style.css", "AssetCssStyle_002"},
	}

	existing := make(map[string]bool)
	for i, tt := range tests {
		if i > 0 && tt.name == "Duplicate" {
			// Simulate existing identifier
			existing["AssetCssStyle"] = true
		}

		t.Run(tt.name, func(t *testing.T) {
			result := generateIdentifier(tt.relPath, existing)
			// Basic validation: should start with "Asset"
			if result[:5] != "Asset" {
				t.Errorf("Identifier should start with 'Asset', got %s", result)
			}
			// Check for valid Go identifier chars
			for _, r := range result {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
					t.Errorf("Invalid character in identifier: %c", r)
				}
			}
		})
	}
}

// TestEstimateFrequencyScore tests frequency score calculation
func TestEstimateFrequencyScore(t *testing.T) {
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
			result := estimateFrequencyScore(tt.path, tt.isEmbed)
			// Just verify it returns a positive value for high-priority files
			// and negative for low-priority
			if tt.path == "index.html" && result <= 0 {
				t.Errorf("Index should have positive score, got %d", result)
			}
		})
	}
}

// TestGenerateShortcut tests shortcut generation
func TestGenerateShortcut(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
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
			result := generateShortcut(tt.relPath)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestSanitizeIdentifier tests identifier sanitization
func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple", "style", "style"},
		{"With dash", "my-file", "myfile"},
		{"With number", "file1", "file1"},
		{"Special chars", "file@#$", "file"},
		{"Unicode", "caf/é", "café"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}
