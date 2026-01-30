// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lynxai-team/emo"
)

// Config - tiny struct that drives the parser behavior.
type Config struct {
	folder string
	mdPath string
	fence  string

	// headerStyle makes the filename appear as
	// --- File: path/to/file.go
	// instead of the default bold style:
	// **path/to/file.go**
	headerStyle string

	// dryRun prevents the Markdown file from being written.
	dryRun bool // if true, nothing is written to disk

	overwrite bool // if false, existing files are left untouched

	all bool // if true, also extract the code blocs without a filename

	reverse bool
}

var log = emo.NewZone("")

// main parses flags, runs the extraction, prints a colored summary.
func main() {
	cfg := newConfig()

	if cfg.reverse {
		err := cfg.genMarkdown()
		if err != nil {
			log.Fatal(err)
		}
		emo.Ok("✅  Markdown generated at", cfg.mdPath)
		return
	}

	err := cfg.extractFiles()
	if err != nil {
		log.Fatal("extract failed", "err", err)
	}
	emo.Ok("✅  Files extracted to", cfg.folder)

	extractedFile, err := collectResults(cfg.folder)
	if err != nil {
		log.Fatal("cannot parse output folder", "err", err)
	}
	printSummary(extractedFile)
}

func newConfig() *Config {
	dryRun := flag.Bool("dry-run", false, "files are not written - useful for tests")
	overwrite := flag.Bool("overwrite", false, "existing files are left untouched (extract mode only)")
	reverse := flag.Bool("reverse", false, "run in reverse mode - generate markdown from a folder tree")
	fence := flag.String("fence", "```", "fence of the code blocs")
	header := flag.String("header", "## File:", "header of the filename line (can be '**' for bold, on '`' for back-quoted)")
	all := flag.Bool("all", false, "also extract the code blocs without a filename")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() > 2 {
		log.Error("too many parameters, max=2", "NArg", flag.NArg())
		usage()
		os.Exit(2)
	}

	cfg := &Config{
		mdPath:      flag.Arg(0),
		folder:      flag.Arg(1),
		fence:       *fence,
		dryRun:      *dryRun,
		overwrite:   *overwrite,
		headerStyle: *header,
		all:         *all,
		reverse:     *reverse,
	}

	if cfg.folder == "" { // set default folder
		if cfg.reverse {
			cfg.folder = "."
			log.Data("parameter folder is not provided => use current working directory")
		} else {
			cfg.folder = "out"
			log.Data(`parameter folder is not provided => use directory "out"`)
		}
	}

	// Normalize folder path
	var err error
	cfg.folder, err = filepath.Abs(cfg.folder)
	if err != nil {
		log.Error("second argument should be a valid directory", "err", err)
		usage()
		os.Exit(2)
	}
	cfg.folder = filepath.Clean(cfg.folder)

	if cfg.mdPath == "" && *reverse {
		cfg.mdPath = filepath.Base(cfg.folder) + ".md"
	}
	if cfg.mdPath == "" {
		usage()
		os.Exit(2)
	}
	return cfg
}

// CLI helpers

// usage prints a short help message.
func usage() {
	prog := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage: %s <markdown-file> [folder]\n", prog)
	flag.PrintDefaults()
}

// extractedFile holds the path (relative to the destination folder)
// and the size of a file that the parser created.
type extractedFile struct {
	path string
	size int64
}

// printSummary outputs a colored checklist of generated files.
func printSummary(results []extractedFile) {
	const (
		green = "\x1b[32m"
		reset = "\x1b[0m"
		check = "✓"
	)

	for _, r := range results {
		// \u202F = narrow no-break space - makes the number line-up nicely.
		fmt.Printf("%s%s %s (%d\u202Fbytes)%s\n", green, check, r.path, r.size, reset)
	}
}

// collectResults walks the destination directory and returns a slice of
// extractedFile (relative path + size).  It is used only for the final
// summary, keeping the parser itself free of bookkeeping.
func collectResults(root string) ([]extractedFile, error) {
	var out []extractedFile
	err := filepath.Walk(root, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// ignore the problematic entry but keep walking
			return nil
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p) // ignore errors - rel will be empty only on a serious bug
		out = append(out, extractedFile{path: rel, size: info.Size()})
		return nil
	})
	return out, err
}

// extractFiles does the real work: it reads *cfg.mdPath*, finds fenced blocks,
// determines the filename from the second line before the opening fence,
// and writes the block to *cfg.folder* according to *cfg*.
func (cfg *Config) extractFiles() error {
	log.Print("Extract code blocs from", cfg.mdPath, "and write the corresponding files in", cfg.folder)
	// -------------------------------------------------------------
	// 1️⃣  Open the markdown file.
	// -------------------------------------------------------------
	f, err := os.Open(cfg.mdPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", cfg.mdPath, err)
	}
	defer f.Close()

	// -------------------------------------------------------------
	// 2️⃣  Prepare a scanner with a generous line buffer.
	// -------------------------------------------------------------
	scanner := bufio.NewScanner(f)

	// -------------------------------------------------------------
	// 3️⃣  Compile the two filename-detection regexes once.
	// -------------------------------------------------------------
	// * case-insensitive
	// * allow letters, digits, hyphens, underscores, slashes and dots.
	headerRe := regexp.MustCompile(`(?i).*\s*File:\s*(.+)`)
	backQuoteRe := regexp.MustCompile("(?i).*`(.+)`[^.]$")
	boldRe := regexp.MustCompile(`(?i)^\*\*\s*(.+)\s*\*\*$`)

	// -------------------------------------------------------------
	// 5️⃣  State used by the simple two-state FSM.
	// -------------------------------------------------------------
	var (
		inBlock    bool     // false → looking for opening fence
		startLine  int      // line number where the current block started
		filename   string   // filename extracted from the second previous line
		bodyLines  []string // lines inside the current block
		lineNumber int      // 1-based line counter
		prev       [2]string
		prevIdx    int
	)

	// -------------------------------------------------------------
	// 6️⃣  Main scanning loop (outside / inside block).
	// -------------------------------------------------------------
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		trim := strings.TrimSpace(line)

		// STATE 0 - we are *outside* a fenced block.
		if !inBlock {
			if strings.HasPrefix(trim, cfg.fence) { // opening fence
				// Check the file format specified by the fence
				// and look at the second previous (two lines before the fence).
				filename = ""
				if len(trim) == len(cfg.fence) {
					emo.ArrowOutf("Skip code bloc starting at line #%d because missing file format (e.g. ```py or ```yaml)", lineNumber)
				} else if m := headerRe.FindStringSubmatch(prev[prevIdx]); m != nil {
					filename = m[1]
					emo.ArrowIn("file", filename, "HEADER", trim, prev[prevIdx])
				} else if m := backQuoteRe.FindStringSubmatch(prev[prevIdx]); m != nil {
					filename = m[1]
					emo.ArrowIn("file", filename, "BACK-QUOTE", trim, prev[prevIdx])
				} else if m := boldRe.FindStringSubmatch(prev[prevIdx]); m != nil {
					filename = m[1]
					emo.ArrowIn("file", filename, "BOLD", trim, prev[prevIdx])
				} else if cfg.all {
					filename = "code-bloc-found-at-line-" + strconv.Itoa(lineNumber) + "." + trim[len(cfg.fence):]
					emo.ArrowIn("file", filename, trim, prev[prevIdx], prev[1-prevIdx])
				} else {
					emo.ArrowOutf("Skip code bloc starting at line #%d because no filename in the second previous line %q %q", lineNumber, prev[prevIdx], prev[1-prevIdx])
				}

				if filename != "" {
					// reset the body collector for the next block
					bodyLines = bodyLines[:0]
					inBlock = true
					startLine = lineNumber
				}
			}
			// Update the two-line look-behind buffer.
			prev[prevIdx] = trim
			prevIdx = 1 - prevIdx
		} else {
			// STATE 1 - we are *inside* a fenced block.
			if trim == cfg.fence { // closing fence
				inBlock = false
				err = cfg.writeBlock(filename, bodyLines)
				if err != nil {
					emo.Warnf("cannot write %q (code block at lines %d-%d) %v", filename, startLine, lineNumber, err)
				} else {
					emo.Ok("File", filename, lineNumber-startLine, "lines")
				}
			} else {
				bodyLines = append(bodyLines, line)
			}
		}
	}

	// -------------------------------------------------------------
	// 7️⃣  Final error handling.
	// -------------------------------------------------------------
	err = scanner.Err()
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	if inBlock {
		return fmt.Errorf("unterminated fenced block starting at line %d", startLine)
	}
	return nil
}

// writeBlock safely writes a single fenced block to disk.
// It respects the Options (dry-run, overwrite) and guarantees that the
// target stays inside *cfg.folder*.
func (cfg *Config) writeBlock(filename string, body []string) error {
	// Resolve the final path and make sure it does not escape the destination.
	target := filepath.Join(cfg.folder, filename)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cfg.folder, cleanTarget)
	if err != nil {
		return fmt.Errorf("filepath %q is not relative to %s: %w", cleanTarget, cfg.folder, err)
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return fmt.Errorf("filename %q starts with ../ (resolves outside of %s)", cleanTarget, cfg.folder)
	}

	// Dry-run: nothing is written.
	if cfg.dryRun {
		return nil
	}

	if !cfg.overwrite {
		_, err = os.Stat(cleanTarget)
		if err == nil {
			emo.Info("File", cleanTarget, "already exists => skip it (overwrite disabled)")
			return nil
		}
		// If Stat returned an error other than “not exists”, let the write fail later.
	}

	// Ensure the directory hierarchy exists.
	dir := filepath.Dir(cleanTarget)
	err = os.MkdirAll(dir, 0o700)
	if err != nil {
		return fmt.Errorf("os.MkdirAll(%s) %w", dir, err)
	}

	// Assemble the file contents (add a trailing newline for niceness).
	content := strings.Join(body, "\n") + "\n"
	err = os.WriteFile(cleanTarget, []byte(content), 0o600)
	if err != nil {
		return fmt.Errorf("os.WriteFile(%s) %w", cleanTarget, err)
	}

	return nil
}

// genMarkdown walks cfg.folder, reads every regular file it finds and
// writes a Markdown document to cfg.mdPath.  The produced file can be fed
// back to ParseFile and will recreate the original files.
//
// The relative path of each file (relative to cfg.folder) is used as the
// identifier - this mirrors the behavior of the extractor, which also
// writes files relative to the destination folder.
func (cfg *Config) genMarkdown() error {
	log.Print("Generate " + cfg.mdPath + " from folder " + cfg.folder)

	if !cfg.overwrite {
		_, err := os.Stat(cfg.mdPath)
		if err == nil {
			return errors.New("File " + cfg.mdPath + " already exists. You may want to use flag -overwrite")
		}
		// If Stat returned an error other than “not exists”, let the write fail later.
	}

	// -----------------------------------------------------------------
	// 1️⃣  Open the destination Markdown file (unless DryRun).
	// -----------------------------------------------------------------
	var out io.Writer
	if cfg.dryRun {
		// Discard output - useful for benchmarking or CI checks.
		out = io.Discard
	} else {
		f, err := os.Create(cfg.mdPath)
		if err != nil {
			return fmt.Errorf("cannot create %s: %w", cfg.mdPath, err)
		}
		defer f.Close()
		out = f
	}
	w := bufio.NewWriter(out)

	// -----------------------------------------------------------------
	// 2️⃣  Walk the source directory tree.
	// -----------------------------------------------------------------
	err := filepath.Walk(cfg.folder, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// Skip the offending entry but continue walking.
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// -------------------------------------------------------------
		// a) Compute the relative path - this is the name that will
		//    appear in the Markdown file.
		// -------------------------------------------------------------
		rel, err := filepath.Rel(cfg.folder, path)
		if err != nil {
			// Should never happen; just skip the file.
			return nil
		}
		rel = filepath.ToSlash(rel) // normalise to forward slashes (Markdown-friendly)

		// -------------------------------------------------------------
		// b) Read the file contents.
		// -------------------------------------------------------------
		data, err := os.ReadFile(path)
		if err != nil {
			// Skip unreadable files - they are not critical for the demo.
			return nil
		}
		// -------------------------------------------------------------
		// c) Emit the filename line.
		// -------------------------------------------------------------
		switch {
		case cfg.headerStyle == "**":
			fmt.Fprintf(w, "**%s**\n\n", rel)
		case len(cfg.headerStyle) == 1:
			fmt.Fprintf(w, "%s%s%s\n\n", cfg.headerStyle, rel, cfg.headerStyle)
		case cfg.headerStyle == "":
			fmt.Fprintf(w, "--- File: %s\n\n", rel)
		default:
			fmt.Fprintf(w, "%s %s\n\n", cfg.headerStyle, rel)
		}

		// -------------------------------------------------------------
		// d) Emit the fenced block.
		// -------------------------------------------------------------
		ext := filepath.Ext(path)
		if ext != "" && ext[0] == '.' {
			ext = ext[1:] // drop the leading dot
		}
		fmt.Fprintf(w, "```%s\n", ext)
		// Write the file content verbatim.
		if len(data) > 0 {
			// Ensure the file ends with a newline - the extractor expects it.
			if data[len(data)-1] != '\n' {
				data = append(data, '\n')
			}
			_, err := w.Write(data)
			if err != nil {
				return err
			}
		}
		fmt.Fprint(w, cfg.fence+"\n\n") // Separate blocks with a blank line for readability.

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", cfg.folder, err)
	}

	// Flush any buffered data to the file (or to /dev/null when DryRun).
	err = w.Flush()
	if err != nil {
		return fmt.Errorf("flush output: %w", err)
	}
	return nil
}
