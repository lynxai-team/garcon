// Copyright 2021 The contributors of Garcon.
// SPDX-License-Identifier: MIT

package main

import (
	"regexp"

	log "github.com/lynxai-team/emo"
)

// matcher holds the compiled regular expressions used to locate a filename
// in the two lines preceding a fenced bloc.  The patterns are ordered from most
// specific to most generic.
type matcher struct {
	exprs [9]*regexp.Regexp // compiled regexes
	prev  [3]string         // buffer with two lines before + one line after
	lang  string            // language tag of the opening fence
	idx   int               // index of the next slot in prev
}

// newMatcher builds a matcher based on the current Config.
func (c *Config) buildMatcher() {
	// The header pattern uses the user‑supplied header text verbatim.
	c.matcher = &matcher{
		exprs: [9]*regexp.Regexp{
			c.custom,
			regexp.MustCompile(`\b[Ff]ile:\s+(` + c.fileRe + `)$`),
			regexp.MustCompile(`^#+\s+(` + c.fileRe + `)$`),
			regexp.MustCompile(`^//\s+(` + c.fileRe + `)$`),
			regexp.MustCompile(`^//\s+(` + c.fileRe + `) - `),
			regexp.MustCompile(`^#+\s+\((` + c.fileRe + `)\)$`),
			regexp.MustCompile("`(" + c.fileRe + ")`[^.]$"),
			regexp.MustCompile("`(" + c.fileRe + ")`$"),
			regexp.MustCompile(`^#*\s*\*\*(` + c.fileRe + `)\*\*`),
		},
	}
}

// store pushes a new line into the search buffer.
func (m *matcher) store(line string) {
	m.prev[m.idx] = line
	m.idx = 1 - m.idx
}

// filename scans the stored lines for a filename using the compiled regexes.
// The first successful match wins.
func (m *matcher) filename() string {
	for _, line := range m.prev {
		if line == "" {
			continue
		}
		for _, ex := range m.exprs {
			matches := ex.FindStringSubmatch(line)
			if len(matches) > 1 {
				log.ArrowIn("file", matches[1], line, ex)
				return matches[1]
			}
		}
		for _, ex := range m.exprs {
			log.Debug("no match", line, ex)
		}
	}
	return ""
}
