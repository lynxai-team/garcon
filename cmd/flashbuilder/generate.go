// Package: main
// Purpose: Code generation, template parsing, data structures
// File: generate.go

package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*.gotmpl
var templateFS embed.FS

// TemplateData aggregates all data for template rendering
type TemplateData struct {
	Config   ConfigData
	Assets   []AssetData
	Dispatch DispatchData
	PathMaps PathMapsData
	MaxLen   int
}

// ConfigData holds configuration for template rendering
type ConfigData struct {
	CSP       string
	HTTPSPort string
	Module    string
}

// AssetData represents an asset for template rendering
type AssetData struct {
	RelPath        string
	Size           int64
	MIME           string
	ETag           string
	Identifier     string
	Filename       string
	IsDuplicate    bool
	CanonicalID    string
	EmbedEligible  bool
	IsIndex        bool
	IsHTML         bool
	FrequencyScore int
	HeaderHTTP     string
	HeaderHTTPS    string
	Variants       []VariantData
}

// VariantData represents a variant for template rendering
type VariantData struct {
	VariantType VariantType
	Size        int64
	HeaderHTTP  string
	HeaderHTTPS string
	Identifier  string
	Extension   string
	CachePath   string
}

// PathMapsData holds path mappings for routing
type PathMapsData struct {
	Canonical map[string]string
	Duplicate map[string]string
	Shortcut  map[string]string
}

// HandlerData holds data for handler template rendering
type HandlerData struct {
	Index    int
	Routes   []RouteData
	Protocol string
}

// cachedTemplates caches parsed templates
var cachedTemplates *template.Template
var errCachedTemplates error

// parseTemplates parses and caches templates
func parseTemplates() (*template.Template, error) {
	if cachedTemplates != nil {
		return cachedTemplates, nil
	}
	if errCachedTemplates != nil {
		return nil, errCachedTemplates
	}

	tmpl := template.New("root").Funcs(funcMap)
	cachedTemplates, errCachedTemplates = tmpl.ParseFS(templateFS, "templates/*.gotmpl")
	return cachedTemplates, errCachedTemplates
}

// funcMap provides template helper functions
var funcMap = template.FuncMap{
	"quote":        func(s string) string { return "\"" + s + "\"" },
	"trim":         strings.TrimSpace,
	"upper":        strings.ToUpper,
	"lower":        strings.ToLower,
	"sanitize":     func(s string) string { return sanitizeIdentifier(s) },
	"escapeHeader": func(s string) string { return strings.ReplaceAll(s, "\n", "\\n") + "\r\n" },
	"add":          func(a, b int) int { return a + b },
	"sub":          func(a, b int) int { return a - b },
	"mul":          func(a, b int) int { return a * b },
	"div":          func(a, b int) int { return a / b },
	"len":          func(s string) int { return len(s) },
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

// renderTemplate renders a template with data
func renderTemplate(name string, data any) (string, error) {
	tmpl, err := parseTemplates()
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	err = tmpl.ExecuteTemplate(&buf, name, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// convertAssets converts asset structures to template data
func convertAssets(assets []asset) []AssetData {
	var result []AssetData
	for _, asset := range assets {
		if asset.IsDuplicate {
			continue
		}

		var variants []VariantData
		for _, v := range asset.Variants {
			variants = append(variants, VariantData{
				VariantType: v.VariantType,
				Size:        v.Size,
				HeaderHTTP:  string(v.HeaderHTTP),
				HeaderHTTPS: string(v.HeaderHTTPS),
				Identifier:  v.Identifier,
				Extension:   v.Extension,
				CachePath:   v.CachePath,
			})
		}

		result = append(result, AssetData{
			RelPath:        asset.RelPath,
			Size:           asset.Size,
			MIME:           asset.MIME,
			ETag:           asset.ETag,
			Identifier:     asset.Identifier,
			Filename:       asset.Filename,
			IsDuplicate:    asset.IsDuplicate,
			CanonicalID:    asset.CanonicalID,
			EmbedEligible:  asset.EmbedEligible,
			IsIndex:        asset.IsIndex,
			IsHTML:         asset.IsHTML,
			FrequencyScore: asset.FrequencyScore,
			HeaderHTTP:     string(asset.HeaderHTTP),
			HeaderHTTPS:    string(asset.HeaderHTTPS),
			Variants:       variants,
		})
	}
	return result
}

// generate generates the Go code for the flash server
func generate(data TemplateData, output string) error {
	// Ensure output directory exists
	if err := os.MkdirAll(output, 0755); err != nil {
		return fmt.Errorf("E099: Failed to create output directory: %v", err)
	}

	// Generate assets/embed.go
	embedCode, err := renderTemplate("embed", data)
	if err != nil {
		return fmt.Errorf("E099: Failed to render embed template: %v", err)
	}
	assetsDir := filepath.Join(output, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return fmt.Errorf("E099: Failed to create assets directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "embed.go"), []byte(embedCode), 0644); err != nil {
		return fmt.Errorf("E099: Failed to write embed.go: %v", err)
	}

	// Generate main.go
	mainCode, err := renderTemplate("main", data)
	if err != nil {
		return fmt.Errorf("E099: Failed to render main template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(output, "main.go"), []byte(mainCode), 0644); err != nil {
		return fmt.Errorf("E099: Failed to write main.go: %v", err)
	}

	return nil
}

// renderHeaderHTTP generates HTTP headers for an asset
func renderHeaderHTTP(asset asset, csp string) []byte {
	var headers strings.Builder

	headers.WriteString(fmt.Sprintf("Content-Type: %s\r\n", asset.MIME))

	if asset.IsIndex {
		headers.WriteString("Cache-Control: public, max-age=31536000, immutable, must-revalidate\r\n")
	} else {
		headers.WriteString("Cache-Control: public, max-age=31536000, immutable\r\n")
	}

	headers.WriteString(fmt.Sprintf("Content-Length: %d\r\n", asset.Size))

	if asset.IsHTML && csp != "" {
		headers.WriteString(fmt.Sprintf("Content-Security-Policy: %s\r\n", csp))
	}

	return []byte(headers.String())
}

// renderHeaderHTTPS generates HTTPS headers for an asset
func renderHeaderHTTPS(asset asset, csp string, httpsPort string) []byte {
	var headers strings.Builder

	headers.WriteString(fmt.Sprintf("Content-Type: %s\r\n", asset.MIME))

	if asset.IsIndex {
		headers.WriteString("Cache-Control: public, max-age=31536000, immutable, must-revalidate\r\n")
	} else {
		headers.WriteString("Cache-Control: public, max-age=31536000, immutable\r\n")
	}

	headers.WriteString(fmt.Sprintf("Content-Length: %d\r\n", asset.Size))

	if asset.IsHTML && csp != "" {
		headers.WriteString(fmt.Sprintf("Content-Security-Policy: %s\r\n", csp))
	}

	headers.WriteString("Strict-Transport-Security: max-age=31536000\r\n")
	headers.WriteString(fmt.Sprintf("Alt-Svc: h3=\":%s\"; ma=2592000\r\n", httpsPort))

	return []byte(headers.String())
}
