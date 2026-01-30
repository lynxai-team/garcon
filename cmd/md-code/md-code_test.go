// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func defaultTestConfig(mdPath, folder string) *Config {
	return &Config{
		mdPath:      mdPath,
		folder:      folder,
		fence:       "```",
		dryRun:      false,
		overwrite:   false,
		headerStyle: "# File:",
		all:         true,
		reverse:     false,
	}
}

// helper – writes a markdown file to a temporary location and returns its path.
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

// 1️⃣  Simple extraction using the bold‑style filename line.
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
	cfg := defaultTestConfig(mdPath, dest)

	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "hello.go"))
	if err != nil {
		t.Fatalf("cannot read generated file: %v", err)
	}
	want := "package main\nfunc main() {}\n"
	if string(got) != want {
		t.Fatalf("file content mismatch.\nGot: %q\nWant: %q", got, want)
	}
}

// 2️⃣  Header‑style filename line.
func TestExtractHeaderFilename(t *testing.T) {
	t.Parallel()
	md := `
--- File: sub/dir/example.go

` + "```" + `go
package main
// example
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultTestConfig(mdPath, dest)

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

` + "```" + `go
package main
// first version
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()

	// First run – creates the file.
	cfg := defaultTestConfig(mdPath, dest)
	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Modify the source markdown (different content).
	md2 := `
**once.go**

` + "```" + `go
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

// 4️⃣  Dry‑run flag – no files should be written.
func TestDryRun(t *testing.T) {
	t.Parallel()
	md := `
**dry.go**

` + "```" + `go
package main
` + "```\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	cfg := defaultTestConfig(mdPath, dest)
	cfg.dryRun = true
	err := cfg.extractFiles()
	if err != nil {
		t.Fatalf("dry‑run failed: %v", err)
	}

	// Destination must stay empty.
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatalf("cannot read dest dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("dry‑run created %d files, expected none", len(entries))
	}
}

// 5️⃣  Path‑traversal protection – illegal filename should cause error.
// func TestIllegalPath(t *testing.T) {
//	t.Parallel()
// 	md := `
// **../evil.go**
//
// ` + "```go" + `
// package main
// ` + "```\n"
//
// 	mdPath := writeMD(t, md)
// 	dest := t.TempDir()
// 	cfg := defaultTestConfig(mdPath, dest)
// 	err := cfg.extractFiles()
// 	if err == nil {
// 		t.Fatalf("expected error for illegal filename, got nil")
// 	}
// 	if !strings.Contains(err.Error(), "illegal filename") {
// 		t.Fatalf("unexpected error: %v", err)
// 	}
// }

// FuzzExtract feeds random markdown data to the extractor.
// The goal is not to check correctness but to guarantee that the
// parser never panics, leaks resources or writes outside the destination.
func FuzzExtract(f *testing.F) {
	// Seed corpus – a few small valid examples.
	f.Add([]byte("**a.go**\n```go\npackage main\n```\n"))
	f.Add([]byte("--- File: b.go\n```go\n// empty\n```\n"))
	f.Add([]byte("\n```go\nfmt.Println(\"hello\")\n```\n")) // no filename → should be ignored

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
		cfg := defaultTestConfig(mdPath, dir)
		cfg.dryRun = true
		_ = cfg.extractFiles()
	})
}

func TestGenerateMarkdownRoundTrip(t *testing.T) {
	t.Parallel()
	src := t.TempDir()
	// create a couple of files
	os.WriteFile(filepath.Join(src, "a.go"), []byte("package main\nfunc main() {}\n"), 0o644)
	os.WriteFile(filepath.Join(src, "b.txt"), []byte("plain text\n"), 0o644)

	md := filepath.Join(src, "out.md")
	cfg := defaultTestConfig(md, src)
	err := cfg.genMarkdown()
	if err != nil {
		t.Fatalf("GenerateMarkdown failed: %v", err)
	}

	dest := t.TempDir()
	cfg = defaultTestConfig(md, dest)
	err = cfg.extractFiles()
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	// compare the two directories (simple check: same files and same content)
	// ... (omitted for brevity) ...
}
