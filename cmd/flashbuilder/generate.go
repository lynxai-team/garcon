// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

//go:embed templates/*.go.gotmpl
var templateFS embed.FS

// templateData aggregates all data for template rendering.
type templateData struct {
	Config   configData
	Assets   []asset
	Dispatch []handlers
	MaxLen   int
}

// configData holds configuration for template rendering.
type configData struct {
	CSP       string
	HTTPSPort string
	Module    string
	Scheme    string // "HTTP" or "HTTPS"
}

// parseTemplates parses and caches templates.
func parseTemplates() (*template.Template, error) {
	tmpl := template.New("root").Funcs(funcMap)
	tmpl, err := tmpl.ParseFS(templateFS, "templates/*.go.gotmpl")
	if err != nil {
		err = fmt.Errorf("E099: Failed to parse templates: %w", err)
	}
	return tmpl, err
}

// funcMap provides template helper functions.
var funcMap = template.FuncMap{
	"quote":        strconv.Quote,
	"trim":         strings.TrimSpace,
	"upper":        strings.ToUpper,
	"lower":        strings.ToLower,
	"escapeHeader": func(s string) string { return strings.ReplaceAll(s, "\n", "\\n") + "\r\n" },
	"add":          func(a, b int) int { return a + b },
	"sub":          func(a, b int) int { return a - b },
	"mul":          func(a, b int) int { return a * b },
	"div":          func(a, b int) int { return a / b },
	"default": func(def, val string) string {
		if val != "" {
			return val
		}
		return def
	},
	"capitalize": func(s string) string {
		if len(s) > 0 {
			return strings.ToUpper(s[:1]) + s[1:]
		}
		return s
	},
}

// renderTemplate renders a template with data.
func renderTemplate(tmpl *template.Template, name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return nil, fmt.Errorf("E099: Failed to render %q template: %w", name, err)
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

	// Generate embed.go
	err = renderWriteCode(dryRun, data, tmpl, output, "embed.go")
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
	err = renderWriteCode(dryRun, data, tmpl, output, "handle.go", "handle-http.go")
	if err != nil {
		return err
	}

	// Generate handle-https.go
	data.Config.Scheme = "HTTPS"
	err = renderWriteCode(dryRun, data, tmpl, output, "handle.go", "handle-https.go")
	if err != nil {
		return err
	}

	// Generate serve-http.go
	data.Config.Scheme = "HTTP"
	err = renderWriteCode(dryRun, data, tmpl, output, "serve.go", "serve-http.go")
	if err != nil {
		return err
	}

	// Generate serve-https.go
	data.Config.Scheme = "HTTPS"
	err = renderWriteCode(dryRun, data, tmpl, output, "serve.go", "serve-https.go")
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

	err := os.MkdirAll(output, 0o755)
	if err != nil {
		return fmt.Errorf("E099: Failed os.MkdirAll(%s): %w", output, err)
	}

	err = os.WriteFile(filepath.Join(output, filename), code, 0o644)
	if err != nil {
		return fmt.Errorf("E099: Failed os.WriteFile(%s/%s): %w", output, filename, err)
	}

	return nil
}
