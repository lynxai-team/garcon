// Package: main
// Purpose: Tests for code generation, template rendering
// File: generate_test.go

package main

import (
	"testing"
)

// TestRenderHeaderHTTP tests HTTP header generation
func TestRenderHeaderHTTP(t *testing.T) {
	tests := []struct {
		name     string
		asset    asset
		csp      string
		contains []string
	}{
		{
			"Simple asset",
			asset{MIME: "text/css", Size: 100, IsIndex: false, IsHTML: false},
			"",
			[]string{"Content-Type: text/css", "Content-Length: 100", "Cache-Control: public, max-age=31536000, immutable"},
		},
		{
			"Index file",
			asset{MIME: "text/html", Size: 500, IsIndex: true, IsHTML: true},
			"default-src 'self'",
			[]string{"Content-Type: text/html", "must-revalidate", "Content-Security-Policy: default-src 'self'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(renderHeaderHTTP(tt.asset, tt.csp))
			for _, expected := range tt.contains {
				if !contains(result, expected) {
					t.Errorf("Expected header to contain %s", expected)
				}
			}
		})
	}
}

// TestRenderHeaderHTTPS tests HTTPS header generation
func TestRenderHeaderHTTPS(t *testing.T) {
	tests := []struct {
		name      string
		asset     asset
		csp       string
		httpsPort string
		contains  []string
	}{
		{
			"Simple asset",
			asset{MIME: "text/css", Size: 100, IsIndex: false, IsHTML: false},
			"",
			"8443",
			[]string{"Content-Type: text/css", "Strict-Transport-Security", "Alt-Svc: h3"},
		},
		{
			"HTML with CSP",
			asset{MIME: "text/html", Size: 500, IsIndex: true, IsHTML: true},
			"default-src 'self'",
			"8443",
			[]string{"Content-Security-Policy", "Strict-Transport-Security"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(renderHeaderHTTPS(tt.asset, tt.csp, tt.httpsPort))
			for _, expected := range tt.contains {
				if !contains(result, expected) {
					t.Errorf("Expected header to contain %s", expected)
				}
			}
		})
	}
}

// TestConvertAssets tests asset data conversion
func TestConvertAssets(t *testing.T) {
	assets := []asset{
		{RelPath: "style.css", Identifier: "AssetStyle", IsDuplicate: false, EmbedEligible: true},
		{RelPath: "dup.css", Identifier: "AssetDup", IsDuplicate: true, CanonicalID: "AssetStyle"},
	}

	result := convertAssets(assets)

	// Should only include non-duplicate assets
	if len(result) != 1 {
		t.Errorf("Expected 1 asset (duplicate excluded), got %d", len(result))
	}

	if result[0].RelPath != "style.css" {
		t.Errorf("Expected style.css, got %s", result[0].RelPath)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
