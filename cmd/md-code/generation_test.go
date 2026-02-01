// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMD helper - writes a markdown file to a temporary location and returns its path.
func writeMD(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "source.md")
	err := os.WriteFile(path, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("cannot write markdown file: %v", err)
	}
	return path
}

// writeFiles helper - creates files in a directory structure.
func writeFiles(t *testing.T, baseDir string, files map[string]string) {
	t.Helper()
	for relPath, content := range files {
		fullPath := filepath.Join(baseDir, relPath)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			t.Fatalf("cannot create directory %s: %v", dir, err)
		}
		err = os.WriteFile(fullPath, []byte(content), 0o644)
		if err != nil {
			t.Fatalf("cannot write file %s: %v", fullPath, err)
		}
	}
}

// assertFileExists helper - asserts that a file exists and has expected content.
func assertFileExists(t *testing.T, path, expectedContent string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read file %s: %v", path, err)
	}
	if string(got) != expectedContent {
		t.Fatalf("file content mismatch for %s.\nGot: %q\nWant: %q", path, got, expectedContent)
	}
}

// assertNoFiles helper - asserts that no files exist in the directory.
func assertNoFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read dest dir %s: %v", dir, err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files in %s, but found %d", dir, len(entries))
	}
}

// 5️⃣  Reverse mode generation.
func TestGenerateMarkdown(t *testing.T) {
	t.Parallel()
	src := t.TempDir()

	// Create test files
	files := map[string]string{
		"a.go":     "package main\nfunc main() {}\n",
		"b.txt":    "plain text\n",
		"sub/c.go": "package main\nfunc hello() {}\n",
	}
	writeFiles(t, src, files)

	md := filepath.Join(src, "out.md")
	c := defaultConfig([]string{"-gen", md, src})
	err := c.generateMarkdown()
	if err != nil {
		t.Fatalf("generateMarkdown failed: %v", err)
	}

	// Verify that the markdown was generated
	_, err = os.Stat(md)
	if err != nil {
		t.Fatalf("markdown file not created: %v", err)
	}

	// Read and verify the markdown content
	content, err := os.ReadFile(md)
	if err != nil {
		t.Fatalf("cannot read generated markdown: %v", err)
	}

	// Check that content includes expected files
	contentStr := string(content)
	if !strings.Contains(contentStr, "a.go") || !strings.Contains(contentStr, "b.txt") || !strings.Contains(contentStr, "sub/c.go") {
		t.Fatalf("generated markdown does not contain expected files:\n%s", contentStr)
	}
}

// 6️⃣  Round-trip.
func TestRoundTrip(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	dirMD := t.TempDir()

	// Create test files
	files := map[string]string{
		"a.go":     "package main\nfunc main() {}\n",
		"b.txt":    "plain text\n",
		"sub/c.go": "package main\nfunc hello() {}\n",
	}
	writeFiles(t, src, files)

	cases := map[string]string{
		"default.md":          defaultHeader,
		"bold.md":             "**",
		"section-bold.md":     "## **",
		"backtick.md":         "`",
		"section-backtick.md": "## `",
		"section-brace.md":    "## (",
	}

	for fileMD, header := range cases {
		t.Run(fileMD, func(t *testing.T) {
			t.Parallel()
			pathMD := filepath.Join(dirMD, fileMD)
			c := defaultConfig([]string{"-gen", "-header", header, pathMD, src})
			if c.header != header {
				t.Fatalf("provided header=%q but got=%q", header, c.header)
			}

			err := c.generateMarkdown()
			if err != nil {
				t.Fatalf("generateMarkdown failed: %v", err)
			}

			dest := t.TempDir()
			c = defaultConfig([]string{pathMD, dest})
			err = c.extractFiles()
			if err != nil {
				t.Fatalf("ParseFile failed: %v", err)
			}

			// Verify that the extracted files match the original
			assertFileExists(t, filepath.Join(dest, "a.go"), files["a.go"])
			assertFileExists(t, filepath.Join(dest, "b.txt"), files["b.txt"])
			assertFileExists(t, filepath.Join(dest, "sub", "c.go"), files["sub/c.go"])
		})
	}
}

// 🆕  FuzzGenerate – fuzz testing for reverse mode.
func FuzzGenerate(f *testing.F) {
	// Seed corpus – valid directory structures
	testFiles := map[string]string{
		"main.go":          "package main\nfunc main() {}\n",
		"utils.go":         "package utils\nfunc helper() {}\n",
		"data/config.json": `{"key": "value"}`,
	}

	// Generate seed data
	for name, content := range testFiles {
		f.Add([]byte(name + "\n" + content))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Create a temporary source directory
		src := t.TempDir()
		writeFiles(t, src, map[string]string{
			"test.go": "package main\nfunc main() {}\n",
		})

		// Create destination markdown file
		md := filepath.Join(src, "output.md")

		// Run reverse mode – any error is acceptable, but it must not panic.
		c := defaultConfig([]string{"-gen", md, src})
		c.dryRun = true
		err := c.generateMarkdown()
		if err != nil {
			// Expected - errors are fine, just don't panic
			return
		}
	})
}
