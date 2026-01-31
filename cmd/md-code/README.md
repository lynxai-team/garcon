# md-code – extract fenced code blocks from Markdown

`md-code` reads a Markdown file, finds fenced code blocks
and writes each block to a separate source file.
The filename is taken from the second previous lines
that appear **just before** the opening fence,
using some patterns such as:

* `**path/to/file.go**` (bold‑style)
* `--- File: path/to/file.go` (header‑style)

Only the standard library and the logger `github.com/lynxai-team/emo` are used.

---

## Table of Contents

* [Run](#run)
* [Command‑line usage](#commandline-usage)
* [Library usage](#library-usage)
* [Options](#options)
* [Testing & fuzzing](#testing--fuzzing)
* [License](#license)

---

## Run

### No install

```bash
go run github.com/lynxai-team/garcon/cmd/md-code@main
```

### Install

```bash
go install github.com/lynxai-team/garcon/cmd/md-code@latest
md-code
```

### From Source

```bash
git clone https://github.com/lynxai-team/garcon.git
cd garcon/cmd/md-code
go build -o md-code .
```

## Command‑line usage

```txt
$ md-code -h
md-code - extract or generate fenced code blocks.

USAGE

  md-code [options]      <markdown-file> [folder]
  md-code [options] -gen [markdown-file] [folder]

If -gen is omitted the program extracts code blocks
from <markdown-file> into [folder] (default "out").

With -gen it creates [markdown-file] from
the files found under [folder] (default ".").
In generation mode, default [markdown-file]
is the folder name with ".md" extension.

EXAMPLES

All these four command lines do the same:
write all src/* files within the src.md file.

  md-code -gen src
  md-code -gen src src.md
  md-code -gen src.md src
  cd src ; md-code -gen

To filter the Go files only:

  md-code -gen -regex '[/A-Za-z0-9_-]+[.]go' src

OPTIONS

  -all
        extract code blocks that have no explicit filename
  -dry-run
        run without writing any files
  -fence string
        fence used to delimit code blocks (must be ≥3 backticks) (default "```")
  -gen
        generate a markdown file from a folder tree
  -header string
        text printed before each generated code block (default "## File: ")
  -overwrite
        overwrite existing files
  -regex string
        regular expression that a filename must match (default "[/A-Za-z0-9._-]*[A-Za-z0-9]")
  -version
        Print version and exit
```

* `markdown-file` – path to the source Markdown file (required).
* `folder` – where the extracted files will be written (or input folder if `-generate`).
  If omitted extract in `out` (if `-generate` use the current directory).
* `-dry-run` – parse the file but **do not write** anything.  
  Useful for testing or when you only want to verify the input.
* `-overwrite` – write a file if it already exists.

### Example

```bash
# extract all blocks from README.md into ./out
md-code README.md out/

# show what would be written without touching the filesystem
md-code README.md out/ -dry-run

# extract but never overwrite files that already exist
md-code README.md out/ -overwrite
```

The program prints a colored checklist after it finishes, e.g.:

    ✓ src/main.go (123 bytes)
    ✓ internal/capsule/capsule.go (4 567 bytes)

## Library usage

The `md-code` tool can also be used as a Go library for programmatic access:

```go
import "github.com/lynxai-team/garcon/cmd/md-code"

// Create a configuration
cfg := &mdcode.Config{
    MdPath:      "documentation.md",
    Folder:      "output",
    Fence:       "```",
    HeaderStyle: "**",
    All:         true,
    DryRun:      false,
    Overwrite:   false,
    Reverse:     false,
}

// Extract code blocks
err := cfg.ExtractFiles()
if err != nil {
    log.Fatal(err)
}

// Or generate Markdown from files
err = cfg.GenMarkdown()
if err != nil {
    log.Fatal(err)
}
```

## Options

| Flag         | Default    | Description                                        |
|--------------|------------|----------------------------------------------------|
| `-all`       | `false`    | Also extract code blocks without detected filename |
| `-dry-run`   | `false`    | Files are not written - useful for tests           |
| `-fence`     |  `` ``` `` | Type of fence (code blocks)                        |
| `-header`    | `## File:` | Header style for filenames                         |
| `-overwrite` | `false`    | Overwrite existing files                           |
| `-generate`  | `false`    | Generate markdown from folder tree                 |

### Supported Filename Styles

md-code recognizes multiple patterns for extracting filename information:

1. **Bold Style**: `**filename.go**`
2. **Backtick Style**: `` `filename.go` ``
3. **File Keyword**: `File: filename.go`
4. **Chapter Style**: `## filename.go`
5. **Custom Style**: `<your-header>filename.go`

### Generated Markdown Format

When generating Markdown from code files, md-code produces:

```markdown
## File: src/main.go

```go
package main

func main() {
    // Your code here
}
```

## File: docs/readme.md

```markdown
# Documentation

Some documentation content...
```

## Testing & fuzzing

The project ships a comprehensive test suite and a simple fuzz target.

```bash
# run unit tests
go test -race -vet all ./...

# run the fuzz target
go test -fuzz=FuzzExtract -run=^$ ./cmd/md-code/
go test -fuzz=FuzzGenerate -run=^$ ./cmd/md-code/
```

The tests cover:

* Both filename syntaxes,
* The `-dry-run` and `-overwrite` behaviors,
* Path-traversal protection,
* Normal happy-path extraction.

The fuzz target feeds random markdown into the parser (with `DryRun:true`) and
asserts that it never crashes.

## Security

md-code includes robust protection against directory traversal attacks:

* Validates file paths against the destination folder
* Rejects filenames containing `../` sequences
* Ensures all generated paths are within the target directory
* Uses safe file system operations
* Implements proper error handling for file I/O

## Performance

### Optimizations

* Efficient regular expression compilation
* Buffered file I/O operations
* Minimal memory allocation
* Fast path resolution

### Large File Support

The tool handles large Markdown files efficiently through:

* Streaming file processing
* Memory-efficient buffer management
* Optimized string operations

## Use Cases

### Documentation Automation

Automatically extract code examples from documentation for testing or distribution.

### Code Sample Management

Maintain consistent code samples across multiple projects and documentation sets.

### Static Site Generation

Support automated generation of documentation sites with embedded code examples.

### Tutorial Creation

Create structured tutorials with embedded code examples that can be extracted and executed.

### Version Control Integration

Keep documentation and code examples in sync automatically.

## Integration

### Git Hooks

Automate extraction of code examples from documentation before commits:

```bash
# .githooks/pre-commit
#!/bin/bash
md-code docs/README.md src/examples/
```

### CI/CD Pipelines

Integrate into build pipelines for documentation generation:

```yaml
# .github/workflows/docs.yml
- name: Generate Documentation
  run: |
    go install github.com/lynxai-team/garcon/cmd/md-code@latest
    md-code -gen -overwrite=true src/ docs.md
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Write tests for new functionality
5. Run `go test` to ensure all tests pass
6. Submit a pull request

## Support

For bugs, feature requests, or questions:

* Open an issue on GitHub
* Submit a pull request
* Contact the maintainers
