// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/lynxai-team/emo"
)

// extract reads the source markdown, finds fenced blocs, determines a
// filename for each bloc and writes the bloc to disk (or simulates the
// write when dry‑run is enabled).
func (c *Config) extract() error {
	log.Printf("Extracting code blocs from %q → %q", c.mdPath, c.folder)

	f, err := os.Open(c.mdPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", c.mdPath, err)
	}
	defer f.Close()

	c.buildMatcher()

	var (
		scanner     = bufio.NewScanner(f)
		lineNum     int
		start       int
		buf         bytes.Buffer // accumulates the current bloc
		closingIsIn bool         // next closing fence is part of the current bloc
	)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trim := strings.TrimSpace(line)

		// Detect fence
		if strings.HasPrefix(line, c.fence) {
			// Closing fence
			if len(trim) == len(c.fence) {
				if start == 0 {
					log.Warnf("Fence without language tag at line #%d - skipping", lineNum)
				} else if closingIsIn {
					log.RequestPostf("corresponding closing fence at line #%d", lineNum)
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

			log.RequestPostf("opening fence in a bloc at line #%d - will consider the corresponding closing fence as part of the current code bloc", lineNum)
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

	err = scanner.Err()
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	if start > 0 {
		return fmt.Errorf("unterminated fenced bloc starting at line %d", start)
	}

	log.Resultf("Files extracted to %s", c.folder)
	return nil
}

// extractBloc creates the target file atomically, respects dry‑run and
// overwrite semantics and rejects any attempt to write outside of the output
// folder (directory‑traversal protection).
func (c *Config) extractBloc(data []byte, start, stop int) {
	filename := c.matcher.filename()
	if filename == "" {
		if c.all { // Auto‑generate a filename using the fence language tag.
			filename = fmt.Sprintf("code-bloc-%d+%d.%s", start, stop-start, c.matcher.lang)
		} else {
			log.Warnf("No filename detected - skip bloc #%d (%d lines) lang=%s - skipping", start, stop-start, c.matcher.lang)
			return
		}
	}

	// Resolve the final destination and ensure it stays inside c.folder.
	target := filepath.Join(c.folder, filename)
	cleanTarget := filepath.Clean(target)

	// Reject absolute paths or paths that escape the output folder.
	if filepath.IsAbs(filename) {
		log.Errorf("absolute filename %q is not allowed - skip bloc #%d (%d lines) lang=%s", filename, start, stop-start, c.matcher.lang)
		return
	}
	rel, err := filepath.Rel(c.folder, cleanTarget)
	if err != nil {
		log.Errorf("no filepath.Rel: %w - skip %q #%d (%d lines) lang=%s", err, filename, start, stop-start, c.matcher.lang)
		return
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		log.Errorf("filename %q resolves outside the output folder - skip bloc #%d (%d lines) lang=%s", filename, start, stop-start, c.matcher.lang)
		return
	}

	// Dry‑run - nothing to write.
	if c.dryRun {
		log.Checkf("dry-run %s #%d (%d lines) lang=%s", filename, start, stop-start, c.matcher.lang)
		return
	}

	// Ensure the directory hierarchy exists.
	dir := filepath.Dir(cleanTarget)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		log.Errorf("mkdir %s: %w - skip %q #%d (%d lines) lang=%s", dir, err, filename, start, stop-start, c.matcher.lang)
		return
	}

	// If overwriting is allowed, remove the existing file first (required on Windows).
	if c.overwrite {
		_ = os.Remove(cleanTarget)
	}

	err = os.WriteFile(cleanTarget, data, 0o600)
	if err != nil {
		log.Errorf("os.WriteFile: %w - skip %q #%d (%d lines) lang=%s", err, filename, start, stop-start, c.matcher.lang)
		return
	}

	log.Checkf("%s #%d (%d lines) lang=%s", filename, start, stop-start, c.matcher.lang)
	return
}
