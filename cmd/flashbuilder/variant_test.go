// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
)

// TestIsCompressible tests MIME type compression eligibility.
func TestIsCompressible(t *testing.T) {
	tests := []struct {
		name     string
		mime     string
		expected bool
	}{
		{"HTML", "text/html", true},
		{"CSS", "text/css", true},
		{"JS", "text/javascript", true},
		{"JSON", "application/json", true},
		{"XML", "application/xml", true},
		{"Image", "image/png", false},
		{"Binary", "application/octet-stream", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCompressible(tt.mime)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsImage tests image MIME type detection.
func TestIsImage(t *testing.T) {
	tests := []struct {
		name     string
		mime     string
		expected bool
	}{
		{"PNG", "image/png", true},
		{"JPEG", "image/jpeg", true},
		{"GIF", "image/gif", true},
		{"SVG", "image/svg+xml", true},
		{"HTML", "text/html", false},
		{"CSS", "text/css", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isImage(tt.mime)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestGenerateVariants_SkipNonEmbed tests that variants are skipped for non-embed assets.
func TestGenerateVariants_SkipNonEmbed(t *testing.T) {
	assets := []asset{
		{RelPath: "large.png", Size: 1000000, EmbedEligible: false, MIME: "image/png"},
		{RelPath: "small.png", Size: 100, EmbedEligible: true, MIME: "image/png"},
	}

	// Run with quality 0 (should skip)
	result := generateVariants(assets, 0, 0, 0, "/tmp/cache")

	// Check that non-embed asset has no variants
	if len(result[0].Variants) > 0 {
		t.Errorf("Non-embed asset should have no variants")
	}

	// Check that embed asset has no variants when quality is 0
	if len(result[1].Variants) > 0 {
		t.Errorf("Asset should have no variants when quality is 0")
	}
}
