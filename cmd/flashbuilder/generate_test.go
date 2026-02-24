// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
)

// TestRenderHeaderHTTP tests HTTP header generation.
func TestRenderHeaderHTTP(t *testing.T) {
	tests := []struct {
		name     string
		csp      string
		contains []string
		asset    asset
	}{
		{
			"Simple asset",
			"",
			[]string{"Content-Type: text/css", "Content-Length: 100", "Cache-Control: public, max-age=31536000, immutable"},
			asset{MIME: "text/css", Size: 100, IsIndex: false, IsHTML: false},
		},
		{
			"Index file",
			"default-src 'self'",
			[]string{"Content-Type: text/html", "must-revalidate", "Content-Security-Policy: default-src 'self'"},
			asset{MIME: "text/html", Size: 500, IsIndex: true, IsHTML: true},
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

// TestRenderHeaderHTTPS tests HTTPS header generation.
func TestRenderHeaderHTTPS(t *testing.T) {
	tests := []struct {
		name      string
		csp       string
		httpsPort string
		contains  []string
		asset     asset
	}{
		{
			"Simple asset",
			"",
			"8443",
			[]string{"Content-Type: text/css", "Strict-Transport-Security", "Alt-Svc: h3"},
			asset{MIME: "text/css", Size: 100, IsIndex: false, IsHTML: false},
		},
		{
			"HTML with CSP",
			"default-src 'self'",
			"8443",
			[]string{"Content-Security-Policy", "Strict-Transport-Security"},
			asset{MIME: "text/html", Size: 500, IsIndex: true, IsHTML: true},
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

// Helper function to check if string contains substring.
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
