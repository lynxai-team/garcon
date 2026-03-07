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
	exprs [11]*regexp.Regexp // compiled regexes
	prev  [5]string          // buffer with two lines before + one line after
	lang  string             // language tag of the opening fence
	idx   int                // index of the next slot in prev
}

// newMatcher builds a matcher based on the current Config.
func newMatcher(custom *regexp.Regexp, fileRe string) *matcher {
	// The header pattern uses the user-supplied header text verbatim.
	return &matcher{
		exprs: [11]*regexp.Regexp{
			custom,
			regexp.MustCompile(`\b[Ff]ile:\s+(` + fileRe + `)$`),
			regexp.MustCompile(`^#+\s+(` + fileRe + `)`),
			regexp.MustCompile("^#+[\\s0-9.]*\\s+`(" + fileRe + ")`"),
			regexp.MustCompile(`^//\s+(` + fileRe + `)$`),
			regexp.MustCompile(`^//\s+(` + fileRe + `) - `),
			regexp.MustCompile(`^#+\s+\((` + fileRe + `)\)$`),
			regexp.MustCompile("`(" + fileRe + ")`[^.]$"),
			regexp.MustCompile("`(" + fileRe + ")`$"),
			regexp.MustCompile(`^#*\s*\*\*(` + fileRe + `)\*\*`),
			regexp.MustCompile(`^\*\*` + "`(" + fileRe + ")`" + `\*\*$`),
		},
	}
}

// store pushes a new line into the search buffer.
func (m *matcher) store(line string) {
	m.prev[m.idx] = line
	m.idx = (m.idx + 1) % len(m.prev)
}

func (m *matcher) reset() {
	for i := range m.prev {
		m.prev[i] = ""
	}
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
				log.ArrowInf("Match file=%s %q %s", matches[1], line, ex)
				return matches[1]
			}
		}
	}

	// report nothing found
	for i, line := range m.prev {
		if line == "" {
			continue
		}
		log.Debugf("No match #%d line=%q", i, line)
	}
	for i, ex := range m.exprs {
		log.Debugf("No match #%d regex=%q", i, ex)
	}

	return ""
}
