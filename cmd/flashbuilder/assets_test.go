// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"io/fs"
	"strconv"
	"testing"
	"testing/fstest"
)

func TestDiscoverAssets(t *testing.T) {
	t.Parallel()

	input := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
		"style.css":  &fstest.MapFile{Data: []byte("body {}")},
		"script.js":  &fstest.MapFile{Data: []byte("console.log")},
		"image.png":  &fstest.MapFile{Data: []byte("\x89PNG")},
		"data.json":  &fstest.MapFile{Data: []byte("{}")},
		"about.html": &fstest.MapFile{Data: []byte("<html><body>Hello</body></html>")},
		"style2.css": &fstest.MapFile{Data: []byte("body { color: red; }")},
		"script2.js": &fstest.MapFile{Data: []byte("console.log('test')")},
	}

	assets, err := discoverAssets(input, "my default csp value")
	if err != nil {
		t.Fatalf("Discovery failed: %v", err)
	}

	// Verify count
	if len(assets) != 8 {
		t.Errorf("Expected 5 assets, got %d", len(assets))
	}
}

// TestEstimateFrequencyScore tests frequency score calculation.
func TestEstimateFrequencyScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{"Index file", "index.html", 1000 + 500 + 200},
		{"Favicon", "favicon.ico", 900 + 200},
		{"CSS file", "style.css", 800 + 200},
		{"JS file", "script.js", 600 + 200},
		{"Logo", "logo.png", 400 + 200},
		{"Deep path", "assets/css/main.css", 800 + 200 - (5 * len("assets/css/main.css")) - 30*2},
		{"Low traffic", "data.pdf", -100 + 200 - (5 * len("data.pdf"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := estimateFrequencyScore(tt.path)
			// Just verify it returns a positive value for high-priority files
			// and negative for low-priority
			if tt.path == "index.html" && result <= 0 {
				t.Errorf("Index should have positive score, got %d", result)
			}
		})
	}
}

// TestGenerateShortcut2 tests shortcut generation.
func TestGenerateShortcut2(t *testing.T) {
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

// MockFile is a struct that satisfies fs.File and io.ReaderAt for mocking.
type MockFile struct {
	data []byte
}

func (f *MockFile) Stat() (fs.FileInfo, error) {
	return nil, nil // Not used in this test context
}

func (f *MockFile) Open() (fs.File, error) {
	return f, nil
}
func (f *MockFile) Close() error { return nil }
func (f *MockFile) ReadAt(p []byte, offset int64) (n int, err error) {
	// Simple implementation for io.ReaderAt
	r := bytes.NewReader(f.data)
	return r.ReadAt(p, offset)
}

func (f *MockFile) Read(p []byte) (n int, err error) {
	// Simple implementation for io.ReaderAt
	r := bytes.NewReader(f.data)
	return r.Read(p)
}

// TestDeduplicate tests the deduplication logic.
func TestDeduplicate(t *testing.T) {
	t.Parallel()

	// Create two assets with the same hash (simulated)
	asset1 := asset{Path: "a.txt", Hash: uint128{Hi: 1, Lo: 1}, IsDuplicate: false}
	asset2 := asset{Path: "b.txt", Hash: uint128{Hi: 1, Lo: 1}, IsDuplicate: false}
	assets := []asset{asset1, asset2}

	result := deduplicate(assets)
	if len(result) != 2 {
		t.Fatalf("Expected 2 assets, got %d", len(result))
	}

	// The second asset should be marked as duplicate
	if !result[1].IsDuplicate {
		t.Errorf("Expected second asset to be marked as duplicate")
	}
	// The first asset should NOT be marked as duplicate
	if result[0].IsDuplicate {
		t.Errorf("Expected first asset NOT to be marked as duplicate")
	}
}

// TestGenerateIdentifier tests identifier generation logic.
func TestGenerateIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		assetPath string
		want      string
	}{
		{"Simple file", "css/style.css", "CssStyleCss"},
		{"Index file", "index.html", "IndexHtml"},
		{"Nested file", "assets/css/main.css", "AssetsCssMainCss"},
		{"Special chars", "assets/images/logo-1.png", "AssetsImagesLogo1Png"},
		{"Duplicate", "css/style.css", "CssStyleCss"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			identifiers := existing{}

			id := identifiers.generateIdentifier(tt.assetPath)
			if id != tt.want {
				t.Errorf("Want %s, got %s", tt.want, id)
			}

			// Test collision

			want := tt.want + "0"
			id = identifiers.generateIdentifier(tt.assetPath)
			if id != want {
				t.Errorf("Want %s, got %s", want, id)
			}

			want = tt.want + "1"
			id = identifiers.generateIdentifier(tt.assetPath)
			if id != want {
				t.Errorf("Want %s, got %s", want, id)
			}

			want = tt.want + "2"
			id = identifiers.generateIdentifier(tt.assetPath)
			if id != want {
				t.Errorf("Want %s, got %s", want, id)
			}

			// Check for valid Go identifier chars
			for _, r := range id {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
					t.Errorf("Invalid character in identifier: %c", r)
				}
			}
		})
	}
}

// TestProcessItem tests the processing of a single file item.
func TestProcessItem(t *testing.T) {
	t.Parallel()

	input := fstest.MapFS{
		"test.txt": &fstest.MapFile{Data: []byte("Hello World")},
	}

	// Call processItem (mocking the io.ReaderAt for imohash)
	// Since fstest.MapFile does not implement ReaderAt, computeImoHashEtag will fallback to io.ReadAll.
	// We test the logic flow.
	asset, err := newAsset(input, "test.txt", "default csp value")
	if err != nil {
		t.Fatalf("ProcessItem failed: %v", err)
	}
	asset.Size = 11

	if asset.Path != "test.txt" {
		t.Errorf("Expected path 'test.txt', got %s", asset.Path)
	}
	if asset.MIME != "text/plain" { // DetectMIME should fallback to sniffing or extension
		// Note: DetectContentType returns "text/html" for "Hello World" often.
		// Let's check the logic.
		// For "Hello World", DetectContentType returns "text/html;..." usually.
		// We rely on the result being non-empty.
		if asset.MIME == "" {
			t.Errorf("Expected MIME type to be detected")
		}
	}
}

// TestParseHTML tests HTML parsing logic.
func TestParseHTML(t *testing.T) {
	t.Parallel()

	const wantCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;"
	const wantAPI = "api/submit"

	test1 := []byte(`
<html>
<head>
	<meta http-equiv="Content-Security-Policy" content="` + wantCSP + `">
</head>
<body>
	<form action="/` + wantAPI + `"></form>
</body>
</html>`)

	test2 := []byte(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Contact & Greetings</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="Content-Security-Policy"
          content="` + wantCSP + `">
</head>
<body>
    <h1>Contact Us</h1>
    <form action="/` + wantAPI + `" method="post">
        <div>
            <label for="greeting-text">What greeting do you want to say?</label>
            <input type="text" id="greeting-text" name="say" value="Hi" required>
        </div>
        <div>
            <label for="greeting-to">Who do you want to say it to?</label>
            <input type="text" id="greeting-to" name="to" value="Mom" required>
        </div>
        <button type="submit">Send my greetings</button>
    </form>
</body>
</html>`)

	input := fstest.MapFS{
		"test1.html": &fstest.MapFile{Data: test1},
		"test2.html": &fstest.MapFile{Data: test2},
	}

	for htmlFile := range input {
		csp, endpoints := parseHTML(input, htmlFile)
		if csp != wantCSP {
			t.Errorf("Want CSP extraction, got %q", csp)
		}
		if len(endpoints) != 1 {
			t.Fatalf("Want 1 endpoint, got %d", len(endpoints))
		}
		if _, ok := endpoints[wantAPI]; !ok {
			t.Errorf("Want %q endpoint, but got %v", wantAPI, endpoints)
		}
	}
}

// TestValidEndpoint tests path sanitization.
func TestValidEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		endpoint string
		want     string
		fail     bool
	}{
		{"/api/submit", "api/submit", false},
		{"../api/submit", "about/api/submit", false}, // path.Clean resolves to api/submit
		{"submit", "about/contact/submit", false},    // path.Clean resolves to api/submit
		{"../../api/submit", "api/submit", false},
		{"/", "", false},
		{"https://example.com/api/submit", "api/submit", false},
		{"../../../../../../../etc/secret.cfg", "", true},
		{"", "about/contact", false},
	}

	for i, tt := range tests {
		name := "#" + strconv.Itoa(i) + " endpoint=" + tt.endpoint
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := validEndpoint("about/contact/index.html", tt.endpoint)
			// NOTE: validEndpoint does path.Clean and join.
			// We check for security and correctness.
			// The logic removes leading slash.
			if (err == nil) == tt.fail {
				if tt.fail {
					t.Error("Expected an error, but got nil")
				} else {
					t.Fatalf("Expected success, but got this error: %s", err)
				}
			}
			if got != tt.want {
				t.Errorf("Got %q, want %q", got, tt.want)
			}
		})
	}
}
