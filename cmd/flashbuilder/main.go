// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/alecthomas/units"
)

type flags struct {
	Input  string `env:"FLASHBUILDER_INPUT"  type:"path" arg:"input"  help:"Path to asset tree"`
	Output string `env:"FLASHBUILDER_OUTPUT" type:"path" arg:"output" help:"Destination for generated files"`

	// Content-Security-Policy header value
	CSP string `env:"FLASHBUILDER_CSP" default:"default-src 'self'"`

	CacheDir    string           `env:"FLASHBUILDER_CACHE_DIR"`
	CacheMax    units.Base2Bytes `env:"FLASHBUILDER_CACHE_MAX"    default:"5GB"`
	EmbedBudget units.Base2Bytes `env:"FLASHBUILDER_EMBED_BUDGET" default:"200GB"`
	Brotli      int              `env:"FLASHBUILDER_BROTLI"       default:"11"`
	AVIF        int              `env:"FLASHBUILDER_AVIF"         default:"50"`
	WebP        int              `env:"FLASHBUILDER_WEBP"         default:"50"`

	Verbosity int  `env:"FLASHBUILDER_LOG_LEVEL" type:"counter" short:"v"`
	DryRun    bool `env:"FLASHBUILDER_DRY_RUN"`
	Test      bool `env:"FLASHBUILDER_TESTS"`
}

func main() {
	var cli flags
	kong.Parse(&cli)
	err := do(&cli)
	if err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

func validateInputs(cli *flags) (*flags, error) {
	var err error

	// Set default cache directory
	if cli.CacheDir == "" {
		cli.CacheDir = getDefaultCacheDir()
	}

	absInput, err := filepath.Abs(cli.Input)
	if err != nil {
		return nil, fmt.Errorf("path.Abs(input) %w", err)
	}

	absOutput, err := filepath.Abs(cli.Output)
	if err != nil {
		return nil, fmt.Errorf("path.Abs(output) %w", err)
	}

	// Security check for input/output equality
	if absInput == absOutput {
		return nil, errors.New("Input and output must differ")
	}

	return cli, nil
}

func do(cli *flags) error {
	cli, err := validateInputs(cli)
	if err != nil {
		return err
	}

	// Use fs.FS for abstraction
	// Convert input path to fs.FS
	input := os.DirFS(cli.Input)

	// Discover assets
	assets, err := discover(input, cli.CSP)
	if err != nil {
		return err
	}

	// Set .Identifier
	setIdentifiers(assets)

	assets = deduplicate(assets)

	// Generate variants
	err = copyAssetsAndVariants(input, assets, cli)
	if err != nil {
		return err
	}

	// Allocate embed budget
	assets = allocateBudget(assets, int64(cli.EmbedBudget))

	// Set frequency scores
	for i := range assets {
		assets[i].Frequency = estimateFrequencyScore(assets[i].Path, assets[i].IsEmbedEligible)
	}

	// Add shortcuts
	assets = addShortcutPaths(assets)

	// Compute MaxLen
	maxLen := computeMaxLen(assets)

	// Generate get and post arrays
	get := buildGet(assets, maxLen)
	post := buildPost(assets, maxLen)

	// Convert to template data
	data := templateData{
		Config: configData{
			CSP:       cli.CSP,
			HTTPSPort: "8443",
			Module:    "flash",
		},
		Assets: assets,
		Get:    get,
		Post:   post,
		MaxLen: maxLen,
	}

	// Generate Go code
	err = generate(data, cli.Output, cli.DryRun)
	if err != nil {
		return err
	}

	if !cli.DryRun {
		err = runGoModInit(cli.Output)
		if err != nil {
			return fmt.Errorf("Failed to run go mod tidy: %w", err)
		}

		err = runGoModTidy(cli.Output)
		if err != nil {
			return fmt.Errorf("Failed to run go mod tidy: %w", err)
		}

		err = runGoBuild(cli.Output)
		if err != nil {
			return fmt.Errorf("Failed to build binary: %w", err)
		}

		if cli.Test {
			err = runTests(cli.Output)
			if err != nil {
				return fmt.Errorf("Test suite failed: %w", err)
			}
		}
	}

	slog.Info("Generation complete")
	return nil
}

// getDefaultCacheDir returns the default cache directory
// Follows XDG Base Directory Specification.
func getDefaultCacheDir() string {
	xdgCache := os.Getenv("XDG_CACHE_HOME")
	if xdgCache != "" {
		return path.Join(xdgCache, "flashbuilder")
	}

	home := os.Getenv("HOME")
	if home != "" {
		return path.Join(home, ".cache", "flashbuilder")
	}

	return ".cache"
}

// runGoModInit runs go mod tidy in the output directory.
func runGoModInit(output string) error {
	cmd := exec.Command("go", "mod", "init", "flash")
	cmd.Dir = output
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runGoModTidy runs go mod tidy in the output directory.
func runGoModTidy(output string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = output
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runGoBuild builds the flash binary.
func runGoBuild(output string) error {
	cmd := exec.Command("go", "build", "-o", "flash")
	cmd.Dir = output
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runTests runs the test suite.
func runTests(output string) error {
	cmd := exec.Command("go", "test", ".", "-race", "-vet", "all")
	cmd.Dir = output
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
