// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"regexp"
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
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{mdPath, dest})

	err := c.extract()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dest, "hello.go"))
	if err != nil {
		t.Fatalf("file has not been extracted: %v", err)
	}
	want := "package main" + "\n" + "func main() {}" + "\n"
	if string(got) != want {
		t.Fatalf("file content mismatch."+
			"\n"+"Got  %q"+
			"\n"+"Want %q", got, want)
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
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{mdPath, dest})

	err := c.extract()
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
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()

	// First run – creates the file.
	c := defaultConfig([]string{mdPath, dest})
	err := c.extract()
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Modify the source markdown (different content).
	md2 := `
**once.go**

` + "```go" + `
package main
// second version – should be ignored
` + "```" + "\n"

	_ = writeMD(t, md2)

	// Second run with Overwrite=false.
	c.overwrite = false
	err = c.extract()
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
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{mdPath, dest})
	c.dryRun = true
	err := c.extract()
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
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{mdPath, dest})

	err := c.extract()
	if err != nil {
		t.Fatalf("should only raise a warning: no need for an error")
	}

	// Verify that no file was created
	assertNoFiles(t, dest)
}

// 8️⃣  All flag – extract blocs without filename.
func TestAllFlag(t *testing.T) {
	t.Parallel()
	md := `
Some text

` + "```go" + `
package main
func main() {}
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{mdPath, dest})
	c.all = true // Enable extraction of blocs without filename

	err := c.extract()
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

	// The filename should contain "code-bloc-4.go"
	if !strings.Contains(entries[0].Name(), "code-bloc-4.go") {
		t.Fatalf("unexpected filename: %s", entries[0].Name())
	}
}

// 9️⃣  No filename blocs – should be ignored when all=false.
func TestNoFilenameIgnored(t *testing.T) {
	t.Parallel()
	md := `
Some text

` + "```go" + `
package main
func main() {}
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{mdPath, dest})
	c.all = false // Disable extraction of blocs without filename

	err := c.extract()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	// Should create no files
	assertNoFiles(t, dest)
}

// 🔟  Empty fence – should skip blocs without language.
func TestEmptyFence(t *testing.T) {
	t.Parallel()
	md := `
**test.go**

` + "```" + `
package main
func main() {}
` + "```" + "\n"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{mdPath, dest})

	err := c.extract()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	// Should create the file
	assertNoFiles(t, dest)
}

func TestExtractSubBlocs(t *testing.T) {
	t.Parallel()
	readme := `This is te README of the project.

Example to use this file:

` + "```go" + `
package main
func main() {}
` + "```" + `

This is the end of the README.
`
	file := "package lib" + "\n" + "func Lib() {}" + "\n"

	md := "**README.md**" +
		"\n" +
		"\n" + "```md" +
		"\n" + readme +
		"```" +
		"\n" +
		"\n" + "## `lib.go`" +
		"\n" + "```go" +
		"\n" + file +
		"```"

	mdPath := writeMD(t, md)
	dest := t.TempDir()
	c := defaultConfig([]string{"-all", mdPath, dest})

	err := c.extract()
	if err != nil {
		t.Fatalf("extractFiles failed: %v", err)
	}

	readmePath := filepath.Join(dest, "README.md")
	filePath := filepath.Join(dest, "lib.go")
	assertFileExists(t, readmePath, readme)
	assertFileExists(t, filePath, file)

	err = os.Remove(readmePath)
	if err != nil {
		t.Errorf("os.Remove(readmePath) %v", err)
	}
	err = os.Remove(filePath)
	if err != nil {
		t.Errorf("os.Remove(filePath) %v", err)
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
	f.Add([]byte("No fenced bloc\n"))
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
		err = c.extract()
		if err != nil {
			// Expected - errors are fine, just don't panic
			return
		}
	})
}

func Test_matcher_filename(t *testing.T) {
	t.Parallel()

	custom, err := regexp.Compile(defaultRegex)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct{ md, want string }{
		{
			`# Complete Truc Implementation

I'll now implement the complete ` + "`truc`" + ` tool following the specification with all clarifications incorporated.

## Project Structure

` + "```" + `
truc/
├── go.mod
├── go.sum
├── main.go
├── version.go
├── templates/
│   ├── embed.gotmpl
│   ├── main.gotmpl
│   └── handlers.gotmpl
└── generated/
    └── flash/
        ├── assets/
        │   └── embed.go
        ├── www/
        │   └── ...
        ├── main.go
        ├── go.mod
        └── go.sum
` + "```" + `

I'll now provide all the source files:

## go.mod

` + "```go" + `
module github.com/org/truc

go 1.26

require (
	github.com/alecthomas/kong v1.14.0
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b
	github.com/google/brotli/go/cbrotli v1.1.0
	github.com/kalafut/imohash v1.1.1
	github.com/kolesa-team/go-webp v1.0.5
	github.com/mtraver/base91 v1.0.0
	github.com/vegidio/avif-go v0.0.0-20260201182506-481b88104109
)
` + "```" + "\n", "go.mod",
		},
	}
	for _, tt := range tests {
		t.Run(tt.md[:10], func(t *testing.T) {
			t.Parallel()

			c := defaultConfig([]string{"in.md", "out/dir"})
			c.dryRun = true
			c.custom = custom
			c.extractFromReader(strings.NewReader(tt.md))
			got := c.matcher.filename()
			if got != tt.want {
				t.Errorf("filename() = %v, want %v", got, tt.want)
			}
		})
	}
}
