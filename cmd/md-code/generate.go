// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	log "github.com/lynxai-team/emo"
)

// ----------------------------------------------------------------------
// Generation mode
// ----------------------------------------------------------------------

// generateMarkdown walks c.folder and writes a markdown document that
// contains each file as a fenced code block.  The output is streamed directly
// to the destination file (or discarded in dry‑run mode) to keep memory usage low.
func (c *Config) generateMarkdown() error {
	log.Printf("Generating markdown %s from folder %s", c.mdPath, c.folder)

	// If the destination already exists and overwriting is disabled, abort early.
	if !c.overwrite {
		_, err := os.Stat(c.mdPath)
		if err == nil {
			return fmt.Errorf("output file %s already exists (use -overwrite to replace)", c.mdPath)
		}
	}

	var out io.Writer
	if c.dryRun {
		out = io.Discard
	} else {
		f, err := os.Create(c.mdPath)
		if err != nil {
			return fmt.Errorf("create %s: %w", c.mdPath, err)
		}
		defer f.Close()
		out = f
	}
	w := bufio.NewWriter(out)

	// Walk the folder tree in lexical order for deterministic output.
	err := filepath.WalkDir(c.folder, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Skip entries that cannot be accessed but continue the walk.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !c.custom.MatchString(path) {
			log.Infof("⚠️  Filename %q does not match regex %q - skipping", path, c.custom)
			return nil
		}
		// Compute a forward‑slash relative path for markdown.
		rel, err := filepath.Rel(c.folder, path)
		if err != nil {
			return nil // should never happen
		}
		rel = filepath.ToSlash(rel)

		// Header line with filename.
		_, err = fmt.Fprint(w, c.genFilenameLine(rel)+"\n\n")
		if err != nil {
			return err
		}

		// Language identifier based on file extension (empty string if unknown).
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		_, err = fmt.Fprintf(w, "%s%s\n", c.fence, ext)
		if err != nil {
			return err
		}

		// Stream file contents into the markdown.
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(w, "error os.Open(%s) %v\n", path, err)
			log.Warnf("error os.Open(%s) %v\n", path, err)
			// If we cannot read a file, just skip it.
			return nil
		} else {
			_, copyErr := io.Copy(w, f)
			closeErr := f.Close()
			if copyErr != nil {
				log.Warnf("error os.Copy %q %v\n", path, copyErr)
			}
			if closeErr != nil {
				log.Warnf("error os.Close %q %v\n", path, closeErr)
			}
		}

		// Ensure the fenced block ends with a newline and a blank line afterwards.
		_, err = fmt.Fprintf(w, "%s\n\n", c.fence)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", c.folder, err)
	}
	err = w.Flush()
	if err != nil {
		return fmt.Errorf("flush output: %w", err)
	}
	return nil
}

// genFilenameLine generates the header line with filename.
func (c *Config) genFilenameLine(filename string) string {
	if c.header == "" {
		return filename
	}

	idx := strings.LastIndexByte(c.header, ' ')

	switch idx {
	// header format = "something "
	case len(c.header) - 1:
		return c.header + filename

	// header format: "`" or "(" or "something `" or "something ("
	case len(c.header) - 2:
		ending := c.header[len(c.header)-1]
		switch ending {
		case '/':
			return c.header + filename
		case '\\':
			return c.header + filename
		case '(':
			ending = ')'
		case '[':
			ending = ']'
		case '{':
			ending = '}'
		}
		return c.header[:idx+1] + filename + string(ending)

	// header format: "**" or "something **"
	case len(c.header) - 3:
		ending := c.header[len(c.header)-2:]
		switch ending {
		case "**":
			return c.header + filename + "**"
		}
		return c.header + filename

	default:
		return c.header + filename // "## File: path/file.go"
	}
}

// ----------------------------------------------------------------------
// Helper utilities
// ----------------------------------------------------------------------

// extractedFile is used only for the final summary printed after extraction.
type extractedFile struct {
	path string
	size int64
}

// collectResults walks the output directory and returns a slice of extractedFile.
// Errors are returned to the caller for proper handling.
func collectResults(root string) ([]extractedFile, error) {
	var out []extractedFile
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Continue walking despite individual errors.
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p) // ignore Rel error - path is under root.
		out = append(out, extractedFile{path: rel, size: info.Size()})
		return nil
	})
	return out, err
}

// printSummary displays a colorized list of extracted files.
// The color codes are emitted only when stdout is a terminal.
func printSummary(results []extractedFile) {
	const (
		green = "\x1b[32m"
		reset = "\x1b[0m"
		check = "✓"
	)
	for _, r := range results {
		// Use a narrow no‑break space (U+202F) to keep the size column aligned.
		fmt.Printf("%s%s %s (%d\u202F"+"bytes)%s\n", green, check, r.path, r.size, reset)
	}
}
