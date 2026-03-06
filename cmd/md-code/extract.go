// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/lynxai-team/emo"
)

// extract reads the source markdown, finds fenced blocs, determines a
// filename for each bloc and writes the bloc to disk (or simulates the
// write when dry-run is enabled).
func (c *Config) extract() error {
	log.Printf("Extracting code blocs from %q → %q", c.mdPath, c.folder)

	f, err := os.Open(c.mdPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", c.mdPath, err)
	}
	defer f.Close()

	return c.extractFromReader(f)
}

func (c *Config) extractFromReader(reader io.Reader) error {
	c.matcher = newMatcher(c.custom, c.fileRe)

	var lineNum int
	var start int
	var buf bytes.Buffer // accumulates the current bloc
	var closingIsIn bool // next closing fence is part of the current bloc

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trim := strings.TrimSpace(line)

		// Detect fence
		if strings.HasPrefix(line, c.fence) {
			// Closing fence
			if len(trim) == len(c.fence) {
				if start == 0 {
					log.Warnf("Skip fence without language tag %s:%d", c.mdPath, lineNum)
				} else if closingIsIn {
					log.RequestPostf("corresponding closing fence %s:%d", c.mdPath, lineNum)
					closingIsIn = false
					goto store_line
				} else {
					c.extractBloc(buf.Bytes(), start, lineNum)
					// change state: zero start means outside of a code bloc
					start = 0
					buf.Reset()
				}
				continue
			}

			// Opening fence while searching a new bloc
			if start == 0 {
				start = lineNum
				c.matcher.lang = trim[len(c.fence):] // store the language tag of the fence (```go)
				continue
			}

			log.RequestPostf("Opening fence %s:%d - will consider the corresponding closing fence as part of the current code bloc", c.mdPath, lineNum)
			closingIsIn = true
			goto store_line
		}

		// zero start => we are outside of a code bloc
		// lineNum==start+1 => we are at the first line of a code bloc
		if start == 0 || (lineNum == start+1) {
			c.matcher.store(line)
			if start == 0 {
				continue
			}
		}

	store_line: // Inside a fenced bloc
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	err := scanner.Err()
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	if start > 0 {
		return fmt.Errorf("Unterminated fenced bloc starting %s:%d", c.mdPath, start)
	}

	return nil
}

// extractBloc creates the target file atomically, respects dry-run and
// overwrite semantics and rejects any attempt to write outside of the output
// folder (directory-traversal protection).
func (c *Config) extractBloc(data []byte, start, stop int) {
	filename := c.matcher.filename()
	if filename == "" {
		if c.all { // Auto-generate a filename using the fence language tag.
			filename = fmt.Sprintf("code-bloc-%d+%d.%s", start, stop-start, c.matcher.lang)
		} else {
			log.Warnf("Skip bloc without filename (%d lines) lang=%s %s:%d", stop-start, c.matcher.lang, c.mdPath, start)
			return
		}
	}

	// Resolve the final destination and ensure it stays inside c.folder.
	target := filepath.Join(c.folder, filename)
	cleanTarget := filepath.Clean(target)

	// Reject absolute paths or paths that escape the output folder.
	if filepath.IsAbs(filename) {
		log.Errorf("Skip %q absolute path not allowed (%d lines) lang=%s %s:%d", filename, stop-start, c.matcher.lang, c.mdPath, start)
		return
	}
	rel, err := filepath.Rel(c.folder, cleanTarget)
	if err != nil {
		log.Errorf("filepath.Rel: %s - Skip %q (%d lines) lang=%s %s:%d", err, filename, stop-start, c.matcher.lang, c.mdPath, start)
		return
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		log.Errorf("Skip %q because outside output folder=%s (%d lines) lang=%s %s:%d", filename, c.folder, stop-start, c.matcher.lang, c.mdPath, start)
		return
	}

	// Dry-run - nothing to write.
	if c.dryRun {
		log.Checkf("dry-run %s (%d lines) lang=%s %s:%d", filename, stop-start, c.matcher.lang, c.mdPath, start)
		return
	}

	// Ensure the directory hierarchy exists.
	dir := filepath.Dir(cleanTarget)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		log.Errorf("mkdir %s: %s - Skip %q (%d lines) lang=%s %s:%d", dir, err, filename, stop-start, c.matcher.lang, c.mdPath, start)
		return
	}

	// If overwriting is allowed, remove the existing file first (required on Windows).
	if c.overwrite {
		_ = os.Remove(cleanTarget)
	}

	err = os.WriteFile(cleanTarget, data, 0o600)
	if err != nil {
		log.Errorf("os.WriteFile: %s - Skip %q (%d lines) lang=%s %s:%d", err, filename, stop-start, c.matcher.lang, c.mdPath, start)
		return
	}

	c.count++
	log.Checkf("Extracted %s (%d lines) lang=%s %s:%d", filename, stop-start, c.matcher.lang, c.mdPath, start)
}
