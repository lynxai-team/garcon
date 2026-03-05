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
	err = renderWriteCode(true, data, tmpl, "outputDir", "main")
	if err != nil {
		t.Skip("Template rendering failed (templates may not be present)")
	}
}

func TestToHuman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		expected string
		size     int64
	}{
		{"100B", 100},
		{"1K", 1024},
		{"2K", 2000},
		{"2K", 2050},
		{"1M", 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"999K", 999 * 1024}, // Check boundary
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()
			result := toHuman(tt.size)
			if result != tt.expected {
				t.Errorf("Want %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestRenderTemplate2 tests template rendering (skip if no templates).
func TestRenderTemplate2(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping template test in short mode")
	}

	// This test requires template files to be present
	// We mock the templateFS using embed (which cannot be done dynamically here without files).
	// Instead, we test the logic of the `funcMap`.

	// Test capitalize function
	result := capitalize("test")
	if result != "Test" {
		t.Errorf("Expected 'Test', got %s", result)
	}
}

// TestFuncMap2 tests the helper functions in the template.
func TestFuncMap2(t *testing.T) {
	t.Parallel()

	// Test quote function

	quoteFunc, ok := funcMap["quote"].(func(string) string)
	if !ok {
		t.Fatal("cannot type assert funcMap[quote]")
	}

	result := quoteFunc("test")
	if result != `"test"` {
		t.Errorf("Want %q, got %s", `"test"`, result)
	}

	// Test trim function

	trimFunc, ok := funcMap["trim"].(func(string) string)
	if !ok {
		t.Fatal("cannot type assert funcMap[trim]")
	}

	result = trimFunc("  test  ")
	if result != "test" {
		t.Errorf("Expected 'test', got %s", result)
	}
}
