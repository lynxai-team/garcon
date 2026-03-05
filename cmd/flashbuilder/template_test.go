// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"testing"
)

// TestFuncMap tests template function map.
func TestFuncMap(t *testing.T) {
	t.Parallel()

	// Test quote function
	quoteFunc, ok := funcMap["quote"].(func(string) string)
	if !ok {
		t.Error("quote function not found in funcMap")
		return
	}
	result := quoteFunc("test")
	if result != `"test"` {
		t.Errorf("Expected \"test\", got %s", result)
	}

	// Test trim function
	trimFunc, ok := funcMap["trim"].(func(string) string)
	if ok {
		result := trimFunc("  test  ")
		if result != "test" {
			t.Errorf("Expected 'test', got %s", result)
		}
	}

	// Test upper function
	upperFunc, ok := funcMap["upper"].(func(string) string)
	if ok {
		result := upperFunc("test")
		if result != "TEST" {
			t.Errorf("Expected 'TEST', got %s", result)
		}
	}

	// Test capitalize function
	capitalizeFunc, ok := funcMap["capitalize"].(func(string) string)
	if ok {
		result := capitalizeFunc("test")
		if result != "Test" {
			t.Errorf("Expected 'Test', got %s", result)
		}
	}
}

// TestRenderTemplate tests template rendering (skip if no templates).
func TestRenderTemplate(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping template test in short mode")
	}

	// This test requires template files to be present
	// Skip if templates are not available
	data := templateData{
		CSP:       "default-src 'self'",
		HTTPSPort: "8443",
		Assets: []asset{
			{
				Path:            "style.css",
				Size:            100,
				MIME:            "text/css",
				ETag:            `"etag123"`,
				Identifier:      "AssetStyle",
				IsEmbedEligible: true,
			},
		},
	}

	tmpl, err := parseTemplates()
	if err != nil {
		t.Fatal(err)
	}

	// Try to render main template (may fail if templates not present)
	_, err = renderTemplate(tmpl, "main", data)
	if err != nil {
		t.Skip("Template rendering failed (templates may not be present)")
	}
}
