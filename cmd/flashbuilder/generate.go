// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"embed"
	"fmt"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
)

//go:embed templates/*.go.gotmpl
var templateFS embed.FS

// templateData aggregates all data for template rendering.
type templateData struct {
	Config  cfg
	Assets  []asset
	Get     []handlers
	Post    []handlers
	MaxLenG int
	MaxLenP int
}

// cfg holds configuration for template rendering.
type cfg struct {
	CSP       string
	HTTPSPort string
	Scheme    string // "HTTP" or "HTTPS"
}

// parseTemplates parses and caches templates.
func parseTemplates() (*template.Template, error) {
	tmpl := template.New("root").Funcs(funcMap)
	tmpl, err := tmpl.ParseFS(templateFS, "templates/*.go.gotmpl")
	if err != nil {
		err = fmt.Errorf("Failed to parse templates: %w", err)
	}
	return tmpl, err
}

// funcMap provides template helper functions.
var funcMap = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"div": func(a, b int) int { return a / b },
	"mul": func(a, b int) int { return a * b },
	"sub": func(a, b int) int { return a - b },

	"lower": strings.ToLower,
	"upper": strings.ToUpper,
	"quote": strconv.Quote,
	"trim":  strings.TrimSpace,

	"human":        toHuman,
	"capitalize":   capitalize,
	"escapeHeader": func(s string) string { return strings.ReplaceAll(s, "\n", "\\n") + "\r\n" },
	"default": func(def, val string) string {
		if val != "" {
			return val
		}
		return def
	},
}

// capitalize uppercases the first character.
func capitalize(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// toHuman converts a size in bytes to a short, human-readable string.
// It uses binary units (1024B=1K, 1024K=1M) and formats
// the mantissa with at most one decimal place.
// Small mantissas (< 8) keep a decimal for better precision.
// Larger values are shown as whole numbers (without decimal).
// If rounding would push the value to the next unit (e.g. 1023.9 K -> 1 M),
// the function automatically promotes the unit.
func toHuman(size int64) string {
	if size < 0 {
		return "0"
	}

	// Unit suffixes, starting with bytes.
	units := []string{"B", "K", "M", "G", "T", "P", "E"}

	// Work with a float so we can keep fractional parts while scaling.
	value := float64(size)
	idx := 0 // index into `units`

	// Scale down until the value fits in the current unit (< 1024) or we
	// have reached the largest defined unit.
	for value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}

	// Determine rounding precision:
	//   * For mantissas < 8 we keep one decimal (e.g. 1.3K)
	//   * Otherwise we round to a whole number (e.g. 8K, 25M, 937G)
	precision := 1.0
	if value < 8 {
		precision = 10 // one-decimal precision
	}
	value = math.Round(value*precision) / precision

	// If rounding caused the mantissa to reach 1024, promote to the next unit.
	if value >= 1024 && idx < len(units)-1 {
		value /= 1024
		idx++
	}

	// `%g` prints the shortest representation, dropping trailing ".0".
	return fmt.Sprintf("%g%s", value, units[idx])
}

// renderTemplate renders a template with data.
func renderTemplate(tmpl *template.Template, name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return nil, fmt.Errorf("Failed to render %q template: %w", name, err)
	}
	return buf.Bytes(), nil
}

// generate generates the Go code for the flash server.
func generate(data templateData, output string, dryRun bool) error {
	tmpl, err := parseTemplates()
	if err != nil {
		return err
	}

	// Generate main.go
	err = renderWriteCode(dryRun, data, tmpl, output, "main.go")
	if err != nil {
		return err
	}

	// Generate assets.go
	err = renderWriteCode(dryRun, data, tmpl, output, "assets.go")
	if err != nil {
		return err
	}

	// Generate headers-http.go
	err = renderWriteCode(dryRun, data, tmpl, output, "headers-http.go")
	if err != nil {
		return err
	}

	// Generate headers-https.go
	err = renderWriteCode(dryRun, data, tmpl, output, "headers-https.go")
	if err != nil {
		return err
	}

	// Generate handle-http.go
	data.Config.Scheme = "HTTP"
	err = renderWriteCode(dryRun, data, tmpl, output, "server.go", "server-http.go")
	if err != nil {
		return err
	}

	// Generate handle-https.go
	data.Config.Scheme = "HTTPS"
	err = renderWriteCode(dryRun, data, tmpl, output, "server.go", "server-https.go")
	if err != nil {
		return err
	}

	// Generate serve-http.go
	data.Config.Scheme = "HTTP"
	err = renderWriteCode(dryRun, data, tmpl, output, "endpoints.go", "endpoints-http.go")
	if err != nil {
		return err
	}

	// Generate serve-https.go
	data.Config.Scheme = "HTTPS"
	err = renderWriteCode(dryRun, data, tmpl, output, "endpoints.go", "endpoints-https.go")
	if err != nil {
		return err
	}

	return nil
}

func renderWriteCode(dryRun bool, data any, tmpl *template.Template, output string, filename ...string) error {
	// render source code
	code, err := renderTemplate(tmpl, filename[0], data)
	if err != nil {
		return err
	}

	goFile := filename[0]
	if len(filename) > 1 {
		goFile = filename[1]
	}

	// create Go file
	err = writeCode(dryRun, code, output, goFile)
	if err != nil {
		return err
	}

	return nil
}

func writeCode(dryRun bool, code []byte, output, filename string) error {
	if dryRun {
		return nil
	}

	err := os.MkdirAll(output, 0o700)
	if err != nil {
		return fmt.Errorf("Failed os.MkdirAll(%s): %w", output, err)
	}

	err = os.WriteFile(path.Join(output, filename), code, 0o600)
	if err != nil {
		return fmt.Errorf("Failed os.WriteFile(%s/%s): %w", output, filename, err)
	}

	return nil
}
