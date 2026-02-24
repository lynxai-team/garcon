// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateCompressionFlags tests compression flag validation.
func TestValidateCompressionFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		brotli    int
		avif      int
		webp      int
		expectErr bool
	}{
		{"Valid Brotli 0-11", 5, 50, 50, false},
		{"Invalid Brotli negative", -1, 50, 50, true},
		{"Invalid Brotli > 11", 12, 50, 50, true},
		{"Valid AVIF 0-100", 5, 75, 50, false},
		{"Invalid AVIF negative", 5, -1, 50, true},
		{"Invalid AVIF > 100", 5, 101, 50, true},
		{"Valid WebP 0-100", 5, 50, 75, false},
		{"Invalid WebP negative", 5, 50, -1, true},
		{"Invalid WebP > 100", 5, 50, 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cli := &cli{
				Brotli: tt.brotli,
				AVIF:   tt.avif,
				WebP:   tt.webp,
			}
			err := validateCompressionFlags(cli)
			if tt.expectErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestGetDefaultCacheDir tests cache directory resolution.
func TestGetDefaultCacheDir(t *testing.T) {
	t.Parallel()

	// Test with XDG_CACHE_HOME
	xdgCache := os.Getenv("XDG_CACHE_HOME")
	home := os.Getenv("HOME")

	// Test XDG_CACHE_HOME set
	if xdgCache != "" {
		expected := filepath.Join(xdgCache, "flashbuilder")
		result := getDefaultCacheDir()
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	}

	// Test HOME set (fallback)
	if home != "" && xdgCache == "" {
		expected := filepath.Join(home, ".cache", "flashbuilder")
		result := getDefaultCacheDir()
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	}

	// Test neither set
	if xdgCache == "" && home == "" {
		expected := ".cache"
		result := getDefaultCacheDir()
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	}
}
