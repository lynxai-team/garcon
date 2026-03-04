// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"path"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
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
		{"Brotli 0-11", 5, 50, 50, false},
		{"Brotli negative", -1, 50, 50, true},
		{"Brotli > 11", 12, 50, 50, true},
		{"AVIF 0-100", 5, 75, 50, false},
		{"AVIF negative", 5, -1, 50, true},
		{"AVIF > 100", 5, 101, 50, true},
		{"WebP 0-100", 5, 50, 75, false},
		{"WebP negative", 5, 50, -1, true},
		{"WebP > 100", 5, 50, 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flags := &flags{
				Input:  t.TempDir(),
				Output: t.TempDir(),
				Brotli: tt.brotli,
				AVIF:   tt.avif,
				WebP:   tt.webp,
			}
			err := do(flags)
			if err != nil {
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
		expected := path.Join(xdgCache, "flashbuilder")
		result := getDefaultCacheDir()
		if result != expected {
			t.Errorf("expected %s, got %s", expected, result)
		}
	}

	// Test HOME set (fallback)
	if home != "" && xdgCache == "" {
		expected := path.Join(home, ".cache", "flashbuilder")
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

func Test_do(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want  string
		flags flags
	}{{`package main

import (
	_ "embed"
)
`, flags{
		Input:       "",
		Output:      "",
		CSP:         "",
		CacheDir:    "",
		EmbedBudget: 0,
		Brotli:      -1,
		AVIF:        -1,
		WebP:        -1,
		Verbosity:   3,
		CacheMax:    5 * 1024 * 1024,
		DryRun:      false,
		Test:        false,
	}}, {`package main

import (
	_ "embed"
)
`, flags{
		Input:       "",
		Output:      "",
		CSP:         "",
		CacheDir:    "",
		EmbedBudget: 1,
		Brotli:      -1,
		AVIF:        -1,
		WebP:        -1,
		Verbosity:   3,
		CacheMax:    5 * 1024 * 1024,
		DryRun:      false,
		Test:        false,
	}}}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()

			tt.flags.Input = t.TempDir()
			os.WriteFile(path.Join(tt.flags.Input, "index.html"), []byte("<html><body>Hey!</body></html>"), 0o600)
			os.WriteFile(path.Join(tt.flags.Input, "favicon.svg"), []byte(`<svg><rect x1="1" x1="2" y1="3" y1="4"/></svg>`), 0o600)

			tt.flags.Output = t.TempDir()
			t.Log("flags.Output = ", tt.flags.Output)

			err := do(&tt.flags)
			if err != nil {
				t.Errorf("expected success but got error=%v", err)
			}

			want := []byte(tt.want)
			got, err := os.ReadFile(path.Join(tt.flags.Output, "assets.go"))
			if err != nil {
				t.Fatalf("Miss assets.go error=%v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("assets.go differ: %v", cmp.Diff(want, got))
				t.Errorf("got:"+"\n"+"%s", got)
			}
		})
	}
}
