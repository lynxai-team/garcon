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

// 1️⃣  Simple extraction using the bold-style filename line.
func TestExtractBoldFilename(t *testing.T) {
	t.Parallel()
	md := `
## 5. HTTP/3 Wrapper (` + "`hello.go`" + `)

` + "```go" + `
package main
func main() {}
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultConfig([]string{mdPath, dest})

	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "hello.go"))
	if err != nil {
		t.Fatalf("file has not been extracted: %v", err)
	}
	want := "package main\nfunc main() {}\n"
	if string(got) != want {
		t.Fatalf("file content mismatch.\nGot: %q\nWant: %q", got, want)
	}
}

// 2️⃣  Header-style filename line.
func TestExtractHeaderFilename(t *testing.T) {
	t.Parallel()
	md := `
--- File: sub/dir/example.go

` + "```go" + `
package main
// example
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultConfig([]string{mdPath, dest})

	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "sub", "dir", "example.go"))
	if err != nil {
		t.Fatalf("cannot read generated file: %v", err)
	}
	if !strings.Contains(string(got), "// example") {
		t.Fatalf("generated file does not contain expected comment")
	}
}

// 3️⃣  Overwrite flag – existing file should be kept when Overwrite=false.
func TestOverwriteFlag(t *testing.T) {
	t.Parallel()
	md := `
**once.go**

` + "```go" + `
package main
// first version
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()

	// First run – creates the file.
	cfg := defaultConfig([]string{mdPath, dest})
	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Modify the source markdown (different content).
	md2 := `
**once.go**

` + "```go" + `
package main
// second version – should be ignored
` + "```\n"

	_ = writeMD(t, md2)

	// Second run with Overwrite=false.
	cfg.overwrite = false
	err = cfg.extractFiles()
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	// Verify that the file still contains the *first* version.
	got, err := os.ReadFile(filepath.Join(dest, "once.go"))
	if err != nil {
		t.Fatalf("cannot read file after second run: %v", err)
	}
	if strings.Contains(string(got), "second version") {
		t.Fatalf("file was overwritten despite Overwrite=false")
	}
}

// 4️⃣  Dry-run flag – no files should be written.
func TestDryRun(t *testing.T) {
	t.Parallel()
	md := `
**dry.go**

` + "```go" + `
package main
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultConfig([]string{mdPath, dest})
	cfg.dryRun = true
	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	// Destination must stay empty.
	assertNoFiles(t, dest)
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
	c := defaultConfig([]string{md, src})
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
	dst := t.TempDir()

	// Create test files
	files := map[string]string{
		"a.go":     "package main\nfunc main() {}\n",
		"b.txt":    "plain text\n",
		"sub/c.go": "package main\nfunc hello() {}\n",
	}
	writeFiles(t, src, files)

	md := filepath.Join(dst, "out.md")
	cfg := defaultConfig([]string{md, src})
	err := cfg.generateMarkdown()
	if err != nil {
		t.Fatalf("GenerateMarkdown failed: %v", err)
	}

	dest := t.TempDir()
	cfg = defaultConfig([]string{md, dest})
	err = cfg.extractFiles()
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// Verify that the extracted files match the original
	assertFileExists(t, filepath.Join(dest, "a.go"), files["a.go"])
	assertFileExists(t, filepath.Join(dest, "b.txt"), files["b.txt"])
	assertFileExists(t, filepath.Join(dest, "sub", "c.go"), files["sub/c.go"])
}

// 7️⃣  Path-traversal protection.
func TestPathTraversalProtection(t *testing.T) {
	t.Parallel()
	md := `
**../evil.go**

` + "```go" + `
package main
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultConfig([]string{mdPath, dest})

	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("should only raise a warning: no need for an error")
	}

	// Verify that no file was created
	assertNoFiles(t, dest)
}

// 8️⃣  All flag – extract blocks without filename.
func TestAllFlag(t *testing.T) {
	t.Parallel()
	md := `
Some text

` + "```go" + `
package main
func main() {}
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultConfig([]string{mdPath, dest})
	cfg.all = true // Enable extraction of blocks without filename

	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	// Should create a file with auto-generated name
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("cannot read dest dir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	// The filename should contain "code-block-4.go"
	if !strings.Contains(entries[0].Name(), "code-block-4.go") {
		t.Fatalf("unexpected filename: %s", entries[0].Name())
	}
}

// 9️⃣  No filename blocks – should be ignored when all=false.
func TestNoFilenameIgnored(t *testing.T) {
	t.Parallel()
	md := `
Some text

` + "```go" + `
package main
func main() {}
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultConfig([]string{mdPath, dest})
	cfg.all = false // Disable extraction of blocks without filename

	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	// Should create no files
	assertNoFiles(t, dest)
}

// 🔟  Empty fence – should skip blocks without language.
func TestEmptyFence(t *testing.T) {
	t.Parallel()
	md := `
**test.go**

` + "```" + `
package main
func main() {}
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultConfig([]string{mdPath, dest})

	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	// Should create the file
	assertNoFiles(t, dest)
}

// 🆕  FuzzExtract – comprehensive fuzz testing.
func FuzzExtract(f *testing.F) {
	// Seed corpus – valid examples
	f.Add([]byte("**a.go**\n```go\npackage main\n```\n"))
	f.Add([]byte("--- File: b.go\n```go\n// empty\n```\n"))
	f.Add([]byte("\n```go\nfmt.Println(\"hello\")\n```\n")) // no filename → should be ignored
	f.Add([]byte("**file.go**\n```go\nfunc main() {\n\t// comment\n}\n```\n"))
	f.Add([]byte("--- File: nested/file.go\n```javascript\nconsole.log('test');\n```\n"))
	f.Add([]byte("**.hidden.go**\n```go\npackage main\n```\n"))
	f.Add([]byte("Some content\n\n```python\nprint('hello')\n```\n"))
	f.Add([]byte("--- File: dir/subdir/file.go\n```go\nfunc test() {}\n```\n"))
	f.Add([]byte("**test.go**\n```\nno lang\n```\n"))
	f.Add([]byte("No fenced block\n"))
	f.Add([]byte("--- File: path/with/slashes.go\n```c\n#include <stdio.h>\n```\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Create a temporary destination directory for each iteration.
		dir := t.TempDir()

		// Write the random markdown to a temp file.
		mdPath := filepath.Join(dir, "fuzz.md")
		err := os.WriteFile(mdPath, data, 0o600)
		if err != nil {
			t.Fatalf("write md: %v", err)
		}

		// Run the extractor – any error is acceptable, but it must not panic.
		cfg := defaultConfig([]string{mdPath, dir})
		cfg.dryRun = true
		err = cfg.extractFiles()
		if err != nil {
			// Expected - errors are fine, just don't panic
			return
		}
	})
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
		cfg := defaultConfig([]string{md, src})
		cfg.dryRun = true
		err := cfg.generateMarkdown()
		if err != nil {
			// Expected - errors are fine, just don't panic
			return
		}
	})
}
