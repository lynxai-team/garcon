// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	c := defaultConfig([]string{mdPath, dest})

	err := c.extractFiles()
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
	c := defaultConfig([]string{mdPath, dest})

	err := c.extractFiles()
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
	c := defaultConfig([]string{mdPath, dest})
	err := c.extractFiles()
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
	c.overwrite = false
	err = c.extractFiles()
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
	c := defaultConfig([]string{mdPath, dest})
	c.dryRun = true
	err := c.extractFiles()
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	// Destination must stay empty.
	assertNoFiles(t, dest)
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
	c := defaultConfig([]string{mdPath, dest})

	err := c.extractFiles()
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
	c := defaultConfig([]string{mdPath, dest})
	c.all = true // Enable extraction of blocks without filename

	err := c.extractFiles()
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
	c := defaultConfig([]string{mdPath, dest})
	c.all = false // Disable extraction of blocks without filename

	err := c.extractFiles()
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
	c := defaultConfig([]string{mdPath, dest})

	err := c.extractFiles()
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
		c := defaultConfig([]string{mdPath, dir})
		c.dryRun = true
		err = c.extractFiles()
		if err != nil {
			// Expected - errors are fine, just don't panic
			return
		}
	})
}
