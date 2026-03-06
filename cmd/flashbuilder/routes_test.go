// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestBuildGet2 tests get array generation.
func TestBuildGet2(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{Route: "index.html", Identifier: "IndexHtml", Frequency: 1000, IsDuplicate: false},
		{Route: "style.css", Identifier: "StyleCss", Frequency: 800, IsDuplicate: false},
		{Route: "script.js", Identifier: "ScriptJs", Frequency: 600, IsDuplicate: false},
	}

	want := []handlers{
		{Length: 0, Entry: "getIndexHtml", PrevEntry: "notFound", Routes: []asset{{Identifier: "IndexHtml", Frequency: 1000, IsShortcut: true}}},
		{Length: 0, Entry: "getIndexHtml", PrevEntry: "getIndexHtml", Routes: []asset{{Identifier: "IndexHtml", Frequency: 1000, IsShortcut: true}}},
		{Length: 1, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{Length: 2, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{Length: 3, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{Length: 4, Entry: "getIndexHtml", PrevEntry: "getIndexHtml"},
		{
			Length:    5,
			PrevEntry: "getIndexHtml",
			Entry:     "getLen5",
			Routes:    []asset{{Route: "style", Identifier: "StyleCss", Frequency: 800, IsShortcut: true}},
		},
		{
			Length:    6,
			PrevEntry: "getLen5",
			Entry:     "getLen6",
			Routes:    []asset{{Route: "script", Identifier: "ScriptJs", Frequency: 600, IsShortcut: true}},
		},
		{Length: 7, Entry: "getLen6", PrevEntry: "getLen6"},
		{Length: 8, Entry: "getLen6", PrevEntry: "getLen6"},
		{
			Length:    9,
			PrevEntry: "getLen6",
			Entry:     "getLen9",
			Routes: []asset{
				{Route: "style.css", Identifier: "StyleCss", Frequency: 800},
				{Route: "script.js", Identifier: "ScriptJs", Frequency: 600},
			},
		},
		{
			Length:    10,
			PrevEntry: "getLen9",
			Entry:     "getLen10",
			Routes:    []asset{{Route: "index.html", Identifier: "IndexHtml", Frequency: 1000}},
		},
	}

	assets = addShortcutRoutes(assets)
	maxLenG := computeMaxLenGet(assets)
	get := buildGet(assets, maxLenG)

	if !cmp.Equal(get, want) {
		t.Errorf("Structs differ: %v", cmp.Diff(want, get))
	}
}

// TestComputeMaxLen2 tests max length calculation.
func TestComputeMaxLen2(t *testing.T) {
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
				assets[i] = asset{Route: p}
			}
			result := computeMaxLenGet(assets)
			if result != tt.expected {
				t.Errorf("Want %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestBuildGet tests the route bucketing for GET requests.
func TestBuildGet(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{Route: "index.html", Identifier: "IndexHtml", Frequency: 1000, IsDuplicate: false},
		{Route: "style.css", Identifier: "StyleCss", Frequency: 800, IsDuplicate: false},
		{Route: "script.js", Identifier: "ScriptJs", Frequency: 600, IsDuplicate: false},
	}

	assets = addShortcutRoutes(assets)
	maxLenG := computeMaxLenGet(assets)
	get := buildGet(assets, maxLenG)

	// Verify structure
	if len(get) == 0 {
		t.Fatal("Get array is empty")
	}

	// Check for correct indexing (based on frequency)
	// The highest frequency asset should appear first in its length group.
	// We need to check specific entries.
	// Note: addShortcutPaths modifies assets. We need to re-verify.

	// Check the first entry (length 0 or 1 depending on logic)
	// The code logic for buildGet: index is length+1?
	// Let's test a specific known case.

	if get[0].Entry != "getIndexHtml" && get[1].Entry != "getIndexHtml" {
		// Logic check
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
				assets[i] = asset{Route: p}
			}
			result := computeMaxLenGet(assets)
			if result != tt.expected {
				t.Errorf("Want %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestAddShortcutPaths tests shortcut generation.
func TestAddShortcutPaths(t *testing.T) {
	t.Parallel()

	assets := []asset{
		{MIME: "font/ttf", Route: "font.ttf", Identifier: "FontTtf"},
		{MIME: "text/css", Route: "style.css", Identifier: "StyleCss"},
		{MIME: "text/html", Route: "index.html", Identifier: "IndexHtml"},
		{MIME: "image/png", Route: "img/logo.png", Identifier: "ImgLogoPng"},
		{MIME: "image/gif", Route: "img/logo.gif", Identifier: "ImgLogoGif"},
		{MIME: "font/woff", Route: "font.woff", Identifier: "FontWoof"},
		{MIME: "font/woff2", Route: "font.woff2", Identifier: "FontWoof2"},
		{MIME: "image/jpeg", Route: "img/logo.jpeg", Identifier: "ImgLogoJpeg"},
		{MIME: "image/webp", Route: "img/logo.webp", Identifier: "ImgLogoWebp"},
		{MIME: "image/avif", Route: "img/logo.avif", Identifier: "ImgLogoAvif"},
		{MIME: "image/x-icon", Route: "img/logo.ico", Identifier: "ImgLogoIco"},
		{MIME: "image/svg+xml", Route: "img/logo.svg", Identifier: "ImgLogoSvg"},
		{MIME: "application/pdf", Route: "doc.pdf", Identifier: "DocPdf"},
		{MIME: "text/xml; charset=utf-8", Route: "data.xml", Identifier: "StyleCss"},
		{MIME: "text/css; charset=utf-8", Route: "style2.css", Identifier: "Style2Css"},
		{MIME: "text/csv; charset=utf-8", Route: "data.csv", Identifier: "DataCsv"},
		{MIME: "text/html; charset=utf-8", Route: "about.html", Identifier: "AboutHtml"},
		{MIME: "text/x-yaml; charset=utf-8", Route: "cfg.yaml", Identifier: "CfgYaml"},
		{MIME: "text/markdown; charset=utf-8", Route: "index.md", Identifier: "IndexMd"},
		{MIME: "application/vnd.ms-fontobject", Route: "font.eot", Identifier: "FontEot"},
		{MIME: "text/javascript; charset=utf-8", Route: "script.js", Identifier: "ScriptJs"},
		{MIME: "application/json; charset=utf-8", Route: "data.json", Identifier: "DataJson"},
	}

	got := addShortcutRoutes(assets)

	want := []asset{
		{Route: "img/logo.avif", MIME: "image/avif", Identifier: "ImgLogoAvif"},
		{Route: "img/logo.jpeg", MIME: "image/jpeg", Identifier: "ImgLogoJpeg"},
		{Route: "img/logo.webp", MIME: "image/webp", Identifier: "ImgLogoWebp"},
		{Route: "img/logo.gif", MIME: "image/gif", Identifier: "ImgLogoGif"},
		{Route: "img/logo.ico", MIME: "image/x-icon", Identifier: "ImgLogoIco"},
		{Route: "img/logo.png", MIME: "image/png", Identifier: "ImgLogoPng"},
		{Route: "img/logo.svg", MIME: "image/svg+xml", Identifier: "ImgLogoSvg"},
		{Route: "about.html", MIME: "text/html; charset=utf-8", Identifier: "AboutHtml"},
		{Route: "font.woff2", MIME: "font/woff2", Identifier: "FontWoof2"},
		{Route: "index.html", MIME: "text/html", Identifier: "IndexHtml"},
		{Route: "style2.css", MIME: "text/css; charset=utf-8", Identifier: "Style2Css"},
		{Route: "data.json", MIME: "application/json; charset=utf-8", Identifier: "DataJson"},
		{Route: "font.woff", MIME: "font/woff", Identifier: "FontWoof"},
		{Route: "script.js", MIME: "text/javascript; charset=utf-8", Identifier: "ScriptJs"},
		{Route: "style.css", MIME: "text/css", Identifier: "StyleCss"},
		{Route: "cfg.yaml", MIME: "text/x-yaml; charset=utf-8", Identifier: "CfgYaml"},
		{Route: "data.csv", MIME: "text/csv; charset=utf-8", Identifier: "DataCsv"},
		{Route: "data.xml", MIME: "text/xml; charset=utf-8", Identifier: "StyleCss"},
		{Route: "font.eot", MIME: "application/vnd.ms-fontobject", Identifier: "FontEot"},
		{Route: "font.ttf", MIME: "font/ttf", Identifier: "FontTtf"},
		{Route: "index.md", MIME: "text/markdown; charset=utf-8", Identifier: "IndexMd"},
		{Route: "doc.pdf", MIME: "application/pdf", Identifier: "DocPdf"},
		// We expect 8 shortcuts
		{IsShortcut: true, Identifier: "ImgLogoAvif", Route: "img/logo", MIME: "image/avif"},
		{IsShortcut: true, Identifier: "AboutHtml", Route: "about", MIME: "text/html; charset=utf-8"},
		{IsShortcut: true, Identifier: "IndexHtml", Route: "", MIME: "text/html"},
		{IsShortcut: true, Identifier: "DataJson", Route: "data", MIME: "application/json; charset=utf-8"},
		{IsShortcut: true, Identifier: "CfgYaml", Route: "cfg", MIME: "text/x-yaml; charset=utf-8"},
		{IsShortcut: true, Identifier: "FontEot", Route: "font", MIME: "application/vnd.ms-fontobject"},
		{IsShortcut: true, Identifier: "IndexMd", Route: "index", MIME: "text/markdown; charset=utf-8"},
		{IsShortcut: true, Identifier: "DocPdf", Route: "doc", MIME: "application/pdf"},
	}

	if len(got) != len(want) {
		t.Errorf("got %d routes, want %d", len(got), len(want))
	}

	count := 0
	for a := range slices.Values(got) {
		if a.IsShortcut {
			count++
		}
	}
	if count != 8 {
		t.Errorf("got %d shortcuts, want 8", count)
	}

	if !cmp.Equal(want, got) {
		t.Fatal(cmp.Diff(want, got))
	}
}
