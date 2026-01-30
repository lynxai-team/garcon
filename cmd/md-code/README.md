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

* [Installation](#installation)
* [Command‑line usage](#command-line-usage)
* [Library usage](#library-usage)
* [Options](#options)
* [Testing & fuzzing](#testing--fuzzing)
* [License](#license)

---

## Run

```bash
go run github.com/lynxai-team/garcon/cmd/md-code@latest
```

or you can install it:

```bash
go get github.com/lynxai-team/garcon/cmd/md-code

md-code
```

## Command‑line usage

```txt
Usage: md-code <markdown-file> [folder]
  -all
        also extract the code blocs without a filename
  -dry-run
        files are not written - useful for tests
  -fence string
        fence of the code blocs (default "```")
  -header string
        header of the filename line (can be '**' for bold, on '`' for back-quoted) (default "## File:")
  -overwrite
        existing files are left untouched (extract mode only)
  -reverse
        run in reverse mode - generate markdown from a folder tree
```

* `markdown-file` – path to the source Markdown file (required).
* `folder` – where the extracted files will be written (or input folder if `-reverse`).
  If omitted extract in `out` (if `-reverse` use the current directory).
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

The program prints a coloured checklist after it finishes, e.g.:

```
✓ src/main.go (123 bytes)
✓ internal/capsule/capsule.go (4 567 bytes)
```

---

## Testing & fuzzing

The project ships a comprehensive test suite and a simple fuzz target.

```bash
# run unit tests
go test -race -vet all ./...

# run the fuzz target (requires Go 1.18+)
go test -fuzz=FuzzExtract -run=^$ ./...
```

The tests cover:

* both filename syntaxes,
* the `-dry-run` and `-overwrite` behaviors,
* path‑traversal protection,
* normal happy‑path extraction.

The fuzz target feeds random markdown into the parser (with `DryRun:true`) and
asserts that it never crashes.
