// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"math"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/sync/errgroup"
)

//go:embed templates/*.go.gotmpl
var templateFS embed.FS

// templateData aggregates all data for template rendering.
type templateData struct {
	outputDir string
	CSP       string
	HTTPSPort string
	Scheme    string // "HTTP" or "HTTPS"
	Assets    []asset
	Get       []handlers
	Post      []handlers
	dryRun    bool
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

// generate generates the Go code for the flash server.
func generate(data templateData) error {
	tmpl, err := parseTemplates()
	if err != nil {
		return err
	}

	// Initialize errgroup with context for cancellation.
	g, ctx := errgroup.WithContext(context.Background())

	// Set the concurrency limit: ensure we do not spawn more than 'workers' goroutines.
	workers := max(2, runtime.NumCPU()/2) // NumCPU = number of logical CPUs
	g.SetLimit(workers)

	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "main.go") })
	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "embed-assets.go") })
	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "headers-http.go") })
	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "headers-https.go") })
	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "routes.go", "routes-http.go") })
	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "routes.go", "routes-https.go") })
	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "handlers-get.go", "handlers-get-http.go") })
	g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "handlers-get.go", "handlers-get-https.go") })

	// do not generate empty Go files (only for web-form submit)
	if len(data.Post) > 0 {
		g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "handlers-post.go", "handlers-post-http.go") })
		g.Go(func() error { return renderWriteCode(ctx, data, tmpl, "handlers-post.go", "handlers-post-https.go") })
	}

	// Wait for all outstanding goroutines to finish.
	// g.Wait returns the first error (if any), canceling the context.
	return g.Wait()
}

func renderWriteCode(ctx context.Context, data templateData, tmpl *template.Template, filename ...string) error {
	templateDefine := filename[0]
	goFile := filename[0]
	if len(filename) > 1 {
		goFile = filename[1]
	}

	localData := data // local copy because we change .Scheme
	if strings.Contains(goFile, "https") {
		localData.Scheme = "HTTPS"
	} else {
		localData.Scheme = "HTTP"
	}

	// render source code from a template with data.
	var code bytes.Buffer
	err := tmpl.ExecuteTemplate(&code, templateDefine, localData)
	if err != nil {
		return fmt.Errorf("Failed to render %q template: %w", templateDefine, err)
	}

	// stop here if --dry-run or if the context is canceled by another worker
	if data.dryRun || (ctx.Err() != nil) {
		return nil
	}

	// ensure output directory exist
	err = os.MkdirAll(data.outputDir, 0o700)
	if err != nil {
		return fmt.Errorf("renderWriteCode MkdirAll %s: %w", data.outputDir, err)
	}

	// write Go file
	goFile = path.Join(data.outputDir, goFile)
	err = os.WriteFile(goFile, code.Bytes(), 0o600)
	if err != nil {
		return fmt.Errorf("renderWriteCode WriteFile %s: %w", goFile, err)
	}

	return nil
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
