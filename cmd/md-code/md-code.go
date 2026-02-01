// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

// md-code is a tiny CLI to:
//
//   - extract fenced code blocks from a Markdown file and write them to a folder tree,
//   - generate a Markdown document that contains all files (from a folder) as fenced blocks.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/lynxai-team/emo"
	"github.com/lynxai-team/garcon/vv"
)

// Default values used when a flag is omitted.
const (
	defaultFence  = "```"
	defaultHeader = "## File: "
	defaultRegex  = "[\\/A-Za-z0-9._-]*[A-Za-z0-9]"

	usage = `md-code - extract or generate fenced code blocks.

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

Insert only the Go files:

  md-code -gen -regex '[/A-Za-z0-9_-]+[.]go' src

The default header is "## File: path/file.go".
This can be changed with -header <text>.

  md-code -gen -header "# "         =>  "# path/file.go"
  md-code -gen -header "## File: "  =>  "## File: path/file.go"

Two special cases in bonus:

  md-code -gen -header "**"  =>  "**path/file.go**"
  md-code -gen -header "` + "`" + `"   =>  "` + "`path/file.go`" + `"

OPTIONS

`
)

// Config holds all runtime options.  Fields are private because the program
// manipulates the struct only internally. See also parseFlags().
type Config struct {
	custom *regexp.Regexp // custom regex to grasp the filename
	fileRe string         // filename regex

	mdPath string // markdown source (extract) or destination (generate)
	folder string // directory where files are read from or written to

	fence  string // fence marker, e.g. "```"
	header string // header prefix used when generating markdown

	// behavioral flags
	all       bool // extract blocks without a detected filename
	dryRun    bool // simulate all write operations
	overwrite bool // allow overwriting existing files
}

// defaultConfig creates a stub configuration for testing.
func defaultConfig(arguments []string) *Config {
	flags := flag.NewFlagSet("md-code", flag.ExitOnError)
	_, c := parseFlags(flags, arguments)
	return c
}

// parseFlags parses command‑line flags, validates them and returns a ready‑to‑use Config.
// It aborts the program with a helpful message on any error.
func parseFlags(flags *flag.FlagSet, arguments []string) (bool, *Config) {
	var (
		fence     = flags.String("fence", defaultFence, "fence used to delimit code blocks (must be ≥3 backticks)")
		header    = flags.String("header", defaultHeader, "text printed before each generated code block")
		regex     = flags.String("regex", defaultRegex, "regular expression that a filename must match")
		all       = flags.Bool("all", false, "extract code blocks that have no explicit filename")
		dryRun    = flags.Bool("dry-run", false, "run without writing any files")
		gen       = flags.Bool("gen", false, "generate a markdown file from a folder tree")
		overwrite = flags.Bool("overwrite", false, "overwrite existing files")
	)
	vv.SetCustomVersionFlag(flags, "", "")
	flags.Usage = func() { fmt.Fprintf(flags.Output(), usage); flags.PrintDefaults() }
	flags.Parse(arguments)

	// Validate fence - the CommonMark spec requires at least three backticks.
	if !strings.HasPrefix(*fence, "```") {
		flags.Usage()
		log.Fatalf("invalid fence %q: must be at least three backticks", *fence)
	}

	// Positional arguments: [markdown-file] [folder]
	if flags.NArg() > 2 {
		flags.Usage()
		log.Fatalf("too many CLI arguments NArg=%d (max=2)", flags.NArg())
	}

	// figure out what argument is a directory
	arg0dir := false // does not exist or not a directory
	arg1dir := false // does not exist or not a directory
	info, err := os.Stat(flags.Arg(0))
	if err == nil && info.IsDir() {
		arg0dir = true // exist and is a directory
	}
	info, err = os.Stat(flags.Arg(1))
	if err == nil && info.IsDir() {
		arg1dir = true // exist and is a directory
	}

	mdPath := flags.Arg(0) // may be empty - defaults are applied later.
	folder := flags.Arg(1) // may be empty - defaults are applied later.
	if arg0dir {
		if arg1dir {
			flags.Usage()
			log.Fatal("both positional arguments are folders - only one folder accepted:", mdPath, folder)
		} else {
			folder, mdPath = mdPath, folder
		}
	}

	// Default folder
	if folder == "" {
		if *gen {
			folder = "."
			log.Data(`Generate mode: default folder is current directory`)
		} else {
			folder = "out"
			log.Data(`Extraction mode: default folder is directory "./out"`)
		}
	}

	// Verify folder path
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		flags.Usage()
		log.Fatalf("invalid folder %q: %v", folder, err)
	}
	absFolder = filepath.Clean(absFolder)

	// generation mode: default markdown name derived from folder
	if *gen && mdPath == "" {
		mdPath = filepath.Base(absFolder) + ".md"
	}

	// Ensure a markdown path is always set
	if mdPath == "" {
		flags.Usage()
		log.Fatal("missing <markdown-file>")
	}

	// Verify markdown file path
	absPath, err := filepath.Abs(mdPath)
	if err != nil {
		flags.Usage()
		log.Fatalf("invalid markdown-file path %q: %v", mdPath, err)
	}
	absPath = filepath.Clean(absPath)

	// extraction mode: compile the filename regex once; abort early on syntax errors.
	expr := *regex
	if !*gen {
		expr = genFilenameLine(regexp.QuoteMeta(*header), *regex)
	}
	custom, err := regexp.Compile(expr)
	if err != nil {
		flags.Usage()
		if !*gen && *header != defaultHeader {
			log.Warnf("verify --header %q", *header)
		}
		if *regex != defaultRegex {
			log.Warnf("verify --regex %q", *regex)
		}
		log.Fatalf("regexp.Compile(%s): %v", expr, err)
	}

	c := &Config{
		mdPath:    absPath,
		folder:    absFolder,
		fence:     *fence,
		header:    *header,
		fileRe:    *regex,
		custom:    custom,
		all:       *all,
		dryRun:    *dryRun,
		overwrite: *overwrite,
	}

	return *gen, c
}

// main entry point.
func main() {
	gen, c := parseFlags(flag.CommandLine, os.Args[1:])

	if gen {
		err := c.generateMarkdown()
		if err != nil {
			log.Fatalf("generation failed: %v", err)
		}
		log.Printf("✅ Markdown generated at %s", c.mdPath)
		return
	}

	err := c.extractFiles()
	if err != nil {
		log.Fatalf("extraction failed: %v", err)
	}
	log.Printf("✅ Files extracted to %s", c.folder)

	results, err := collectResults(c.folder)
	if err != nil {
		log.Fatalf("cannot walk output folder: %v", err)
	}
	printSummary(results)
}
