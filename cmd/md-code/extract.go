// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/lynxai-team/emo"
)

// matcher holds the compiled regular expressions used to locate a filename
// in the two lines preceding a fenced block.  The patterns are ordered from most
// specific to most generic.
type (
	namedRegex struct {
		name string
		re   *regexp.Regexp
	}
	matcher struct {
		exprs []namedRegex // compiled regexes
		prev  [2]string    // two‑line look‑behind buffer
		idx   int          // index of the next slot in prev
	}
)

// newMatcher builds a matcher based on the current Config.
func (c *Config) newMatcher() *matcher {
	// The header pattern uses the user‑supplied header text verbatim.
	return &matcher{
		exprs: []namedRegex{
			{"Custom", c.custom},
			{"File", regexp.MustCompile(`\b[Ff]ile:\s+(` + c.fileRe + `)$`)},
			{"Chapter", regexp.MustCompile(`^#+\s+(` + c.fileRe + `)$`)},
			{"BackQuote", regexp.MustCompile("`(" + c.fileRe + ")`[^.]$")},
			{"Bold", regexp.MustCompile(`^\*\*(` + c.fileRe + `)\*\*`)},
		},
	}
}

// store pushes a new line into the look‑behind buffer.
func (m *matcher) store(line string) {
	m.prev[m.idx] = line
	m.idx = 1 - m.idx
}

// filename scans the stored lines for a filename using the compiled regexes.
// The first successful match wins.
func (m *matcher) filename(fence string) string {
	for _, line := range m.prev {
		if line == "" {
			continue
		}
		for _, ex := range m.exprs {
			matches := ex.re.FindStringSubmatch(line)
			if len(matches) > 1 {
				log.ArrowIn("file", matches[1], ex.name, fence, line)
				return matches[1]
			}
		}
	}
	return ""
}

// ----------------------------------------------------------------------
// Extraction mode
// ----------------------------------------------------------------------

// extractFiles reads the source markdown, finds fenced blocks, determines a
// filename for each block and writes the block to disk (or simulates the
// write when dry‑run is enabled).
func (c *Config) extractFiles() error {
	log.Printf("Extracting code blocks from %q → %q", c.mdPath, c.folder)

	f, err := os.Open(c.mdPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", c.mdPath, err)
	}
	defer f.Close()

	var (
		scanner         = bufio.NewScanner(f)
		matcher         = c.newMatcher()
		lineNum         int
		startLine       int
		filename        string
		buf             bytes.Buffer // accumulates the current block
		skipNextClosing bool
	)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trim := strings.TrimSpace(line)

		// Detect fence
		if strings.HasPrefix(line, c.fence) {
			// Closing fence
			if len(trim) == len(c.fence) {
				if filename == "" {
					log.Infof("⚠️  Fence without language tag at line #%d - skipping", lineNum)
				} else if skipNextClosing {
					skipNextClosing = false
				} else {
					err = c.writeBlock(filename, buf.Bytes())
					if err != nil {
						log.Printf("⚠️  Failed to write %q (lines %d‑%d): %v", filename, startLine, lineNum, err)
					} else {
						log.Printf("✅ Written %s (%d lines)", filename, lineNum-startLine)
					}
					// change state: empty filename means outside of a code bloc
					filename = ""
					buf.Reset()
				}
				continue
			}

			// Opening fence while searching a new bloc
			if filename == "" {
				if filename = matcher.filename(trim); filename != "" {
					// Success: we just inferred the filename from the preceding lines.
				} else if c.all {
					// Auto‑generate a filename using the fence language tag.
					filename = fmt.Sprintf("code-block-%d.%s", lineNum, trim[len(c.fence):])
				} else {
					log.Printf("⚠️  No filename detected for block starting at line %d - skipping", lineNum)
					continue
				}
				startLine = lineNum
				continue
			}

			log.Infof("⚠️  Found an opening fence in a bloc at line #%d - will skip the corresponding closing fence", lineNum)
			skipNextClosing = true
			continue
		}

		// empty filename => we are outside of a code bloc
		if filename == "" {
			// Update the look‑behind buffer for the next iteration.
			matcher.store(line)
			continue
		}

		// Inside a fenced block
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	err = scanner.Err()
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}
	if filename != "" {
		return fmt.Errorf("unterminated fenced block starting at line %d", startLine)
	}
	return nil
}

// writeBlock creates the target file atomically, respects dry‑run and
// overwrite semantics and rejects any attempt to write outside of the output
// folder (directory‑traversal protection).
func (c *Config) writeBlock(name string, data []byte) error {
	// Resolve the final destination and ensure it stays inside c.folder.
	target := filepath.Join(c.folder, name)
	cleanTarget := filepath.Clean(target)

	// Reject absolute paths or paths that escape the output folder.
	if filepath.IsAbs(name) {
		return fmt.Errorf("absolute filename %q is not allowed", name)
	}
	rel, err := filepath.Rel(c.folder, cleanTarget)
	if err != nil {
		return fmt.Errorf("cannot compute relative path: %w", err)
	}
	if strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return fmt.Errorf("filename %q resolves outside the output folder", name)
	}

	// Ensure the directory hierarchy exists.
	dir := filepath.Dir(cleanTarget)
	err = os.MkdirAll(dir, 0o755)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// Dry‑run - nothing to write.
	if c.dryRun {
		return nil
	}

	// Write to a temporary file first, then rename atomically.
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	// In case of any error, clean up the temporary file.
	defer func() {
		tmp.Close()
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()

	_, err = tmp.Write(data)
	if err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	err = tmp.Sync()
	if err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// If overwriting is allowed, remove the existing file first (required on Windows).
	if c.overwrite {
		_ = os.Remove(cleanTarget)
	}

	err = os.Rename(tmp.Name(), cleanTarget)
	if err != nil {
		return fmt.Errorf("rename %s → %s: %w", tmp.Name(), cleanTarget, err)
	}
	return nil
}
