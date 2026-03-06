// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/go-cmp/cmp"

	_ "embed"
)

//go:embed images/logo-flash.avif
var imageAVIF []byte

//go:embed images/logo-flash.jpg
var imageJPG []byte

//go:embed images/logo-flash.png
var imagePNG []byte

//go:embed images/logo-flash.webp
var imageWebP []byte

// tinyPNG is a minimal 1×1 black PNG image (67 bytes).
var tinyPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
	0x00, 0x00, 0x00, 0x0D, // IHDR header length = 13 ()
	0x49, 0x48, 0x44, 0x52, // "IHDR"
	0x00, 0x00, 0x00, 0x01, // width  = 1
	0x00, 0x00, 0x00, 0x01, // height = 1
	0x08, 0x02, 0x00, 0x00, 0x00, // bit depth (8), color (RGB), compression, filter, interlace
	0x90, 0x77, 0x53, 0xDE, // CRC for IHDR
	0x00, 0x00, 0x00, 0x0A, // IDAT data length = 10
	0x49, 0x44, 0x41, 0x54, // "IDAT"
	0x78, 0x9C, 0x63, 0x60, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, // zlib‑compressed scanline: filter byte (0) + RGB = (0,0,0)
	0xE5, 0x27, 0xD4, 0x5A, // CRC for IDAT
	0x00, 0x00, 0x00, 0x00, // image trailer length = 0
	0x49, 0x45, 0x4E, 0x44, // "IEND"
	0xAE, 0x42, 0x60, 0x82, // CRC for IEND
}

// TestIsCompressible tests MIME type compression eligibility.
func TestIsCompressible(t *testing.T) {
	t.Parallel()

	type expected struct{ b, a, w bool }
	tests := []struct {
		expected expected
		name     string
		mime     string
	}{
		{expected{true, false, false}, "JS", "text/javascript"},
		{expected{true, false, false}, "XML", "application/xml"},
		{expected{true, false, false}, "CSS", "text/css"},
		{expected{true, false, false}, "Markdown", "text/markdown"},
		{expected{true, false, false}, "SVG", "image/svg+xml"},
		{expected{true, false, false}, "HTML", "text/html"},
		{expected{true, false, false}, "JSON", "application/json"},
		{expected{false, true, true}, "JPEG", "image/jpeg"},
		{expected{false, true, true}, "PNG", "image/png"},
		{expected{false, true, true}, "GIF", "image/gif"},
		{expected{false, true, true}, "AVIF", "image/avif"},
		{expected{false, true, true}, "WebP", "image/webp"},
		{expected{false, false, false}, "Binary", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b, a, w := variantEligibility(tt.mime)
			if b != tt.expected.b || a != tt.expected.a || w != tt.expected.w {
				t.Errorf("Want %v, got b=%v a=%v w=%v", tt.expected, b, a, w)
			}
		})
	}
}

// TestIsImage tests image MIME type detection.
func TestIsImage(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			result := isImage(tt.mime)
			if result != tt.expected {
				t.Errorf("Want %v, got %v", tt.expected, result)
			}
		})
	}
}

// isImage determines if content is an image.
func isImage(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// TestGenerateVariants_SkipDuplicates tests that variants are skipped for duplicated assets.
func TestGenerateVariants_SkipDuplicates(t *testing.T) {
	t.Parallel()

	textFile := []byte("This is a text file ")
	for range 9000 {
		textFile = append(textFile, []byte("\n"+"another line in this text file")...)
	}

	input := fstest.MapFS{
		"text.txt":      &fstest.MapFile{Data: textFile},
		"large.png":     &fstest.MapFile{Data: imagePNG},
		"duplicate.png": &fstest.MapFile{Data: imagePNG},
		"small.png":     &fstest.MapFile{Data: tinyPNG},
		"medium.jpeg":   &fstest.MapFile{Data: imageJPG},
		"medium.webp":   &fstest.MapFile{Data: imageWebP},
		"medium.avif":   &fstest.MapFile{Data: imageAVIF},
	}

	assets := []asset{
		{Path: "text.txt", MIME: "text/plain", IsEmbedEligible: true, IsDuplicate: false, Size: 16_000},
		{Path: "large.png", MIME: "image/png", IsEmbedEligible: false, IsDuplicate: false, Size: 1000_000},
		{Path: "small.png", MIME: "image/png", IsEmbedEligible: true, IsDuplicate: false, Size: 1000},
		{Path: "duplicate.png", MIME: "image/png", IsEmbedEligible: true, IsDuplicate: true, Size: 1000_000},
		{Path: "medium.jpeg", MIME: "image/jpeg", IsEmbedEligible: true, IsDuplicate: false, Size: 5000},
		{Path: "medium.webp", MIME: "image/webp", IsEmbedEligible: true, IsDuplicate: false, Size: 5000},
		{Path: "medium.avif", MIME: "image/avif", IsEmbedEligible: true, IsDuplicate: false, Size: 5000},
	}
	expected := []asset{
		{VariantExt: ".br", Path: "text.txt", MIME: "text/plain", Size: 53, IsEmbedEligible: true},
		{VariantExt: ".avif", Path: "large.png", MIME: "image/png", Size: 1034},
		{VariantExt: "", Path: "small.png", MIME: "image/png", Size: 1000, IsEmbedEligible: true},
		{VariantExt: "", Path: "duplicate.png", MIME: "image/png", Size: 1000_000, IsEmbedEligible: true, IsDuplicate: true},
		{VariantExt: ".avif", Path: "medium.jpeg", MIME: "image/jpeg", Size: 727, IsEmbedEligible: true},
		{VariantExt: "", Path: "medium.webp", MIME: "image/webp", Size: 5000, IsEmbedEligible: true},
		{VariantExt: ".avif", Path: "medium.avif", MIME: "image/avif", Size: 1037, IsEmbedEligible: true},
	}

	cli := flags{
		Input:    t.TempDir(),
		Output:   t.TempDir(),
		CacheDir: t.TempDir(),
		CacheMax: 99_000_000,
		Brotli:   5,
		AVIF:     50,
		WebP:     50,
	}

	err := linkCopyAssetsVariants(input, assets, &cli)
	if err != nil {
		t.Errorf("Unexpected err=%s", err)
	}

	if !cmp.Equal(expected, assets) {
		t.Error(cmp.Diff(expected, assets))
	}
}

// TestAllocateBudget2 tests embed budget allocation.
func TestAllocateBudget2(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{Route: "small.txt", Size: 100},
		{Route: "medium.txt", Size: 500},
		{Route: "large.txt", Size: 1000},
	}

	// Budget of 600 should fit small + medium, but not large
	allocateEmbedBudget(assets, 600)

	// Verify sorting (smallest first)
	if len(assets) < 3 {
		t.Fatalf("Expected 3 assets, got %d", len(assets))
	}

	// First two should be eligible
	if !assets[0].IsEmbedEligible {
		t.Errorf("Asset 0 should be eligible")
	}
	if !assets[1].IsEmbedEligible {
		t.Errorf("Asset 1 should be eligible")
	}

	// Last one should not be eligible
	if assets[2].IsEmbedEligible {
		t.Errorf("Asset 2 should not be eligible (too large)")
	}
}

// TestCleanCache2 tests cache cleaning.
func TestCleanCache2(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create test files with different ages
	files := []struct {
		name    string
		modTime time.Time
	}{
		{"old.txt", time.Now().Add(-time.Hour)},
		{"new.txt", time.Now()},
	}

	var maxSize int64
	for _, f := range files {
		fp := path.Join(tmpDir, f.name)
		err := os.WriteFile(fp, []byte(f.name), 0o600)
		if err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
		// Set modification time
		err = os.Chtimes(fp, f.modTime, f.modTime)
		if err != nil {
			t.Fatalf("Failed to set mod time: %v", err)
		}
		maxSize = max(maxSize, int64(len(f.name)))
	}

	// Clean cache to remove oldest file (simulate small max size)
	cleanCache(tmpDir, maxSize) // 0 bytes max = force deletion

	// Old file should be deleted
	_, err := os.Stat(path.Join(tmpDir, "old.txt"))
	if err == nil {
		t.Error("Old file should have been deleted")
	}

	// New file should still exist
	_, err = os.Stat(path.Join(tmpDir, "new.txt"))
	if err != nil {
		t.Error("New file should still exist")
	}
}

// TestVariantEligibility tests MIME type compression eligibility.
func TestVariantEligibility(t *testing.T) {
	t.Parallel()

	type expected struct{ b, a, w bool }
	tests := []struct {
		expected expected
		name     string
		mime     string
	}{
		{expected{true, false, false}, "JS", "text/javascript"},
		{expected{true, false, false}, "XML", "application/xml"},
		{expected{true, false, false}, "CSS", "text/css"},
		{expected{true, false, false}, "Markdown", "text/markdown"},
		{expected{true, false, false}, "SVG", "image/svg+xml"},
		{expected{true, false, false}, "HTML", "text/html"},
		{expected{true, false, false}, "JSON", "application/json"},
		{expected{false, true, true}, "JPEG", "image/jpeg"},
		{expected{false, true, true}, "PNG", "image/png"},
		{expected{false, true, true}, "GIF", "image/gif"},
		{expected{false, true, true}, "AVIF", "image/avif"},
		{expected{false, true, true}, "WebP", "image/webp"},
		{expected{false, false, false}, "Binary", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b, a, w := variantEligibility(tt.mime)
			if b != tt.expected.b || a != tt.expected.a || w != tt.expected.w {
				t.Errorf("Want %v, got b=%v a=%v w=%v", tt.expected, b, a, w)
			}
		})
	}
}

// TestAllocateBudget tests embed budget allocation.
func TestAllocateBudget(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{Route: "small.txt", Size: 100},
		{Route: "medium.txt", Size: 500},
		{Route: "large.txt", Size: 1000},
	}

	// Budget of 600 should fit small + medium, but not large
	allocateEmbedBudget(assets, 600)

	// Verify sorting (smallest first)
	if len(assets) < 3 {
		t.Fatalf("Expected 3 assets, got %d", len(assets))
	}

	// First two should be eligible
	if !assets[0].IsEmbedEligible {
		t.Errorf("Asset 0 should be eligible")
	}
	if !assets[1].IsEmbedEligible {
		t.Errorf("Asset 1 should be eligible")
	}

	// Last one should not be eligible
	if assets[2].IsEmbedEligible {
		t.Errorf("Asset 2 should not be eligible (too large)")
	}
}

// TestCleanCache tests cache cleaning logic.
func TestCleanCache(t *testing.T) {
	t.Parallel()

	// Create temp directory
	tmpDir := t.TempDir()

	// Create a file
	filePath := path.Join(tmpDir, "test.txt")
	err := os.WriteFile(filePath, []byte("test"), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	// Clean cache with maxSize = 0 (should delete everything)
	cleanCache(tmpDir, 0)

	// Check if file exists
	_, err = os.Stat(filePath)
	if err == nil {
		t.Error("File should have been deleted")
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
				t.Errorf("Want %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestCopyAssetsAndVariants tests the variant generation logic (mocked).
// NOTE: This test requires CGO dependencies. It's marked as an integration test.
func TestIntegration_Variants(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup mock filesystem
	input := fstest.MapFS{
		"test.txt": &fstest.MapFile{Data: []byte("This is a text file")},
	}

	assets := []asset{
		{Route: "test.txt", MIME: "text/plain", IsEmbedEligible: true, Size: 16_000},
	}

	cli := flags{
		Input:    t.TempDir(),
		Output:   t.TempDir(),
		CacheDir: t.TempDir(),
		CacheMax: 99_000_000,
		Brotli:   5, // Enable Brotli
	}

	// This will call copyAssetsAndVariants
	// We expect a .br variant to be created (or skipped if logic says so)
	err := linkCopyAssetsVariants(input, assets, &cli)
	if err != nil {
		t.Errorf("Unexpected err=%s", err)
	}
}
