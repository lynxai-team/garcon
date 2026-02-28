// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/alecthomas/units"
)

// CLI structure.
type cli struct {
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

	Verbosity int `env:"FLASHBUILDER_LOG_LEVEL" type:"counter" short:"v"`

	DryRun bool `env:"FLASHBUILDER_DRY_RUN"`
	Test   bool `env:"FLASHBUILDER_TESTS"`
}

func main() {
	// Kong handles --help flag and environment variables automatically via tags
	var cli cli
	kong.Parse(&cli)
	err := do(&cli)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func setCacheDir(dir string) (string, error) {
	// Set default cache directory
	if dir == "" {
		dir = getDefaultCacheDir()
	}

	// Ensure cache directory exists
	err := ensureCacheDir(dir)
	if err != nil {
		return "", fmt.Errorf("E099: Failed to create cache directory: %w", err)
	}

	return dir, nil
}

func validateInputs(cli *cli) (*cli, error) {
	var err error

	cli.CacheDir, err = setCacheDir(cli.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("E099: Failed to create cache directory: %w", err)
	}

	absInput, err := filepath.Abs(cli.Input)
	if err != nil {
		return nil, fmt.Errorf("E099: filepath.Abs(input) %w", err)
	}

	absOutput, err := filepath.Abs(cli.Output)
	if err != nil {
		return nil, fmt.Errorf("E099: filepath.Abs(output) %w", err)
	}

	if absInput == absOutput {
		return nil, errors.New("E099: Input and output must differ")
	}

	return cli, nil
}

func do(cli *cli) error {
	cli, err := validateInputs(cli)
	if err != nil {
		return err
	}

	// Discover assets
	assets, err := discover(cli.Input, cli.CSP)
	if err != nil {
		return err
	}

	// Set .Identifier and .Filename
	assets = setIdentifiers(assets)

	assets, err = computeHashesETags(assets)
	if err != nil {
		return err
	}

	assets = deduplicate(assets)

	// Generate variants
	assets = generateVariants(assets, cli.Brotli, cli.AVIF, cli.WebP, cli.CacheDir)

	// Clean cache to maintain size limit
	err = cleanCache(cli.CacheDir, int64(cli.CacheMax))
	if err != nil {
		return err
	}

	// Allocate embed budget
	assets = allocateBudget(assets, int64(cli.EmbedBudget))

	// Set frequency scores
	for i := range assets {
		assets[i].Frequency = estimateFrequencyScore(assets[i].RelPath, assets[i].EmbedEligible)
	}

	// Create links
	if !cli.DryRun {
		err = createLinks(assets, cli.Output, cli.CacheDir)
		if err != nil {
			return fmt.Errorf("E087: Failed to create links: %w", err)
		}
	}

	// Add shortcuts
	assets = addShortcutPaths(assets)

	// Compute MaxLen
	maxLen := computeMaxLen(assets)

	// Generate get array
	get := buildGet(assets, maxLen)

	// Convert to template data
	data := templateData{
		Config: configData{
			CSP:       cli.CSP,
			HTTPSPort: "8443",
			Module:    "flash",
		},
		Assets: assets,
		Get:    get,
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
			return fmt.Errorf("E099: Failed to run go mod tidy: %w", err)
		}

		err = runGoModTidy(cli.Output)
		if err != nil {
			return fmt.Errorf("E099: Failed to run go mod tidy: %w", err)
		}

		err = runGoBuild(cli.Output)
		if err != nil {
			return fmt.Errorf("E099: Failed to build binary: %w", err)
		}

		if cli.Test {
			err = runTests(cli.Output)
			if err != nil {
				return fmt.Errorf("E079: Test suite failed: %w", err)
			}
		}
	}

	fmt.Println("Generation complete")
	return nil
}

// getDefaultCacheDir returns the default cache directory
// Follows XDG Base Directory Specification.
func getDefaultCacheDir() string {
	xdgCache := os.Getenv("XDG_CACHE_HOME")
	if xdgCache != "" {
		return filepath.Join(xdgCache, "flashbuilder")
	}

	home := os.Getenv("HOME")
	if home != "" {
		return filepath.Join(home, ".cache", "flashbuilder")
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
