# md-code-lite

A lightweight tool that can:
- Convert markdown with code blocks to individual source files
- Convert source files to markdown documentation

## Installation

You can install `md-code-lite` using Go:

```bash
go install github.com/lynxai-team/garcon/cmd/md-code-lite@latest
```

## Commands

### `tocode` - Convert markdown to source files

Extract code blocks from a markdown file and save them as individual source files.

```bash
md-code-lite tocode -i input.md -o output-directory
```

### `tomd` - Convert source files to markdown

Generate a markdown file from source files with proper syntax highlighting.

```bash
# From a directory
md-code-lite tomd -d source-directory -o output.md

# From specific files
md-code-lite tomd -f file1.go -f file2.js -f file3.css -o output.md

# Combine both
md-code-lite tomd -d src -f extra.txt -o combined.md
```
