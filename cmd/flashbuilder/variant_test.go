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

//go:embed test-data/images/logo.avif
var imageAVIF []byte

//go:embed test-data/images/logo.jpg
var imageJPG []byte

//go:embed test-data/images/logo.png
var imagePNG []byte

//go:embed test-data/images/logo.webp
var imageWebP []byte

// tinyPNG is a minimal 1×1 black PNG image (67 bytes).
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
	0x78, 0x9C, 0x63, 0x60, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, // zlib-compressed scanline: filter byte (0) + RGB = (0,0,0)
	0xE5, 0x27, 0xD4, 0x5A, // CRC for IDAT
	0x00, 0x00, 0x00, 0x00, // image trailer length = 0
	0x49, 0x45, 0x4E, 0x44, // "IEND"
	0xAE, 0x42, 0x60, 0x82, // CRC for IEND
}

// TestIsCompressible tests MIME type compression eligibility.
func TestIsCompressible(t *testing.T) {
	t.Parallel()

	type want struct{ b, a, w bool }
	//nolint:govet // do not optimize test functions
	tests := []struct {
		want want
		name string
		mime string
	}{
		{want{true, true, true}, "ICO", "image/x-icon"},
		{want{false, true, true}, "avif", "image/avif"},
		{want{false, true, true}, "AVIF", "image/avif"},
		{want{false, true, true}, "gif", "image/gif"},
		{want{false, true, true}, "GIF", "image/gif"},
		{want{false, true, true}, "jpeg", "image/jpeg"},
		{want{false, true, true}, "JPEG", "image/jpeg"},
		{want{false, true, true}, "png", "image/png"},
		{want{false, true, true}, "PNG", "image/png"},
		{want{false, true, true}, "WebP", "image/webp"},
		{want{true, false, false}, "CSS", "text/css; charset=utf-8"},
		{want{true, false, false}, "css", "text/css"},
		{want{true, false, false}, "CSS", "text/css"},
		{want{true, false, false}, "CSV", "text/csv; charset=utf-8"},
		{want{true, false, false}, "HTML", "text/html; charset=utf-8"},
		{want{true, false, false}, "html", "text/html"},
		{want{true, false, false}, "HTML", "text/html"},
		{want{true, false, false}, "JS", "text/javascript; charset=utf-8"},
		{want{true, false, false}, "JS", "text/javascript"},
		{want{true, false, false}, "JSON", "application/json; charset=utf-8"},
		{want{true, false, false}, "JSON", "application/json"},
		{want{true, false, false}, "Markdown", "text/markdown; charset=utf-8"},
		{want{true, false, false}, "Markdown", "text/markdown"},
		{want{true, false, false}, "PDF", "application/pdf"},
		{want{true, false, false}, "SVG", "image/svg+xml"},
		{want{true, false, false}, "SVG", "image/svg+xml"},
		{want{true, false, false}, "XML", "application/xml"},
		{want{true, false, false}, "XML", "text/xml; charset=utf-8"},
		{want{true, false, false}, "YAML", "text/x-yaml; charset=utf-8"},
		{want{false, false, false}, "woff", "font/woff"},
		{want{false, false, false}, "Binary", "application/octet-stream"},
		{want{false, false, false}, "woff2", "font/woff2"},
		{want{false, false, false}, "ms-font", "application/vnd.ms-fontobject"},
		{want{false, false, false}, "ttf", "font/ttf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b, a, w := variantEligibility(tt.mime)
			if b != tt.want.b || a != tt.want.a || w != tt.want.w {
				t.Errorf("Want %v, got b=%v a=%v w=%v", tt.want, b, a, w)
			}
		})
	}
}

// TestIsImage tests image MIME type detection.
func TestIsImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mime string
		want bool
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
			got := isImage(tt.mime)
			if got != tt.want {
				t.Errorf("Want %v, got %v", tt.want, got)
			}
		})
	}
}

// isImage determines if content is an image.
func isImage(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

func TestLinkCopyAssetsVariants(t *testing.T) {
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
		{MIME: "text/plain", IsEmbedEligible: true, Path: "text.txt", Size: 16_000},
		{MIME: "image/png", IsEmbedEligible: false, Path: "large.png", Size: 1000_000},
		{MIME: "image/png", IsEmbedEligible: true, Path: "small.png", Size: 1000},
		{MIME: "image/png", IsEmbedEligible: true, Path: "duplicate.png", Size: 1000_000, IsDuplicate: true},
		{MIME: "image/jpeg", IsEmbedEligible: true, Path: "medium.jpeg", Size: 5000},
		{MIME: "image/webp", IsEmbedEligible: true, Path: "medium.webp", Size: 5000},
		{MIME: "image/avif", IsEmbedEligible: true, Path: "medium.avif", Size: 5000},
	}
	expected := []asset{
		{MIME: "text/plain", IsEmbedEligible: true, Path: "text.txt", Size: 53, VariantExt: ".br"},
		{MIME: "image/png", IsEmbedEligible: false, Path: "large.png", Size: 1034, VariantExt: ".avif"},
		{MIME: "image/png", IsEmbedEligible: true, Path: "small.png", Size: 1000, VariantExt: ""},
		{MIME: "image/png", IsEmbedEligible: true, Path: "duplicate.png", Size: 1000_000, VariantExt: "", IsDuplicate: true},
		{MIME: "image/jpeg", IsEmbedEligible: true, Path: "medium.jpeg", Size: 727, VariantExt: ".avif"},
		{MIME: "image/webp", IsEmbedEligible: true, Path: "medium.webp", Size: 727, VariantExt: ".avif"},
		{MIME: "image/avif", IsEmbedEligible: true, Path: "medium.avif", Size: 727, VariantExt: ".avif"},
	}

	cli := flags{
		InDir:    t.TempDir(),
		OutDir:   t.TempDir(),
		CacheDir: t.TempDir(),
		CacheMax: 99_000_000,
		Brotli:   5,
		AVIF:     55,
		WebP:     55,
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
		{Path: "small.txt", Size: 100},
		{Path: "medium.txt", Size: 500},
		{Path: "large.txt", Size: 1000},
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
	//nolint:govet // do not optimize test functions
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

	type want struct{ b, a, w bool }
	//nolint:govet // do not optimize test functions
	tests := []struct {
		want want
		name string
		mime string
	}{
		{want{true, false, false}, "JS", "text/javascript"},
		{want{true, false, false}, "XML", "application/xml"},
		{want{true, false, false}, "CSS", "text/css"},
		{want{true, false, false}, "Markdown", "text/markdown"},
		{want{true, false, false}, "SVG", "image/svg+xml"},
		{want{true, false, false}, "HTML", "text/html"},
		{want{true, false, false}, "JSON", "application/json"},
		{want{false, true, true}, "JPEG", "image/jpeg"},
		{want{false, true, true}, "PNG", "image/png"},
		{want{false, true, true}, "GIF", "image/gif"},
		{want{false, true, true}, "AVIF", "image/avif"},
		{want{false, true, true}, "WebP", "image/webp"},
		{want{false, false, false}, "Binary", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b, a, w := variantEligibility(tt.mime)
			if b != tt.want.b || a != tt.want.a || w != tt.want.w {
				t.Errorf("Want %v, got b=%v a=%v w=%v", tt.want, b, a, w)
			}
		})
	}
}

// TestAllocateBudget tests embed budget allocation.
func TestAllocateBudget(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{Path: "small.txt", Size: 100},
		{Path: "medium.txt", Size: 500},
		{Path: "large.txt", Size: 1000},
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
		name   string
		inPath string
		want   string
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
			got := generateShortcut(tt.inPath)
			if got != tt.want {
				t.Errorf("Want %s, got %s", tt.want, got)
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
		{Path: "test.txt", MIME: "text/plain", IsEmbedEligible: true, Size: 16_000},
	}

	cli := flags{
		InDir:    t.TempDir(),
		OutDir:   t.TempDir(),
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
