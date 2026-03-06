// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/alecthomas/units"
)

type flags struct {
	InDir  string `env:"FLASHBUILDER_INPUT_DIR"  type:"path" arg:"input"  help:"Path to asset tree"`
	OutDir string `env:"FLASHBUILDER_OUTPUT_DIR" type:"path" arg:"output" help:"Destination for generated files"`

	// Content-Security-Policy header value
	CSP string `env:"FLASHBUILDER_CSP" default:"default-src 'self'"`

	CacheDir    string           `env:"FLASHBUILDER_CACHE_DIR"`
	CacheMax    units.Base2Bytes `env:"FLASHBUILDER_CACHE_MAX"    default:"5GB"`
	EmbedBudget units.Base2Bytes `env:"FLASHBUILDER_EMBED_BUDGET" default:"200GB"`
	Brotli      int              `env:"FLASHBUILDER_BROTLI"       default:"11"`
	AVIF        int              `env:"FLASHBUILDER_AVIF"         default:"50"`
	WebP        int              `env:"FLASHBUILDER_WEBP"         default:"50"    name:"webp"`

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

	cli.InDir, err = filepath.Abs(cli.InDir)
	if err != nil {
		return nil, fmt.Errorf("path.Abs(input) %w", err)
	}

	cli.OutDir, err = filepath.Abs(cli.OutDir)
	if err != nil {
		return nil, fmt.Errorf("path.Abs(output) %w", err)
	}

	// Security check for input/output equality
	if cli.InDir == cli.OutDir {
		return nil, errors.New("Input and output must differ")
	}

	return cli, nil
}

func do(cli *flags) error {
	setLogLevel(cli.Verbosity)

	cli, err := validateInputs(cli)
	if err != nil {
		return err
	}

	// use fs.FS to access the assets files
	// this simplifies the tests mocking (fstest)
	input := os.DirFS(cli.InDir)

	return process(input, cli)
}

func process(input fs.FS, cli *flags) error {
	assets, err := discoverAssets(input, cli.CSP)
	if err != nil {
		return err
	}

	generateIdentifiers(assets)
	assets = deduplicate(assets)
	allocateEmbedBudget(assets, int64(cli.EmbedBudget))

	// copy assets to assets/ and www/
	// generate assets variants (Brotli, AVIF, WebP)
	// replace assets by their variants
	err = linkCopyAssetsVariants(input, assets, cli)
	if err != nil {
		return err
	}

	assets = addShortcutRoutes(assets)

	get, post, err := buildGetPostDispatch(assets)
	if err != nil {
		return err
	}

	// Generate Go code
	data := templateData{
		outputDir: cli.OutDir,
		dryRun:    cli.DryRun,
		CSP:       cli.CSP,
		HTTPSPort: "8443",
		Assets:    assets,
		Get:       get,
		Post:      post,
	}
	err = generate(data)
	if err != nil {
		return err
	}

	if !cli.DryRun {
		err = runGoModInit(cli.OutDir)
		if err != nil {
			return fmt.Errorf("Failed to run go mod tidy: %w", err)
		}

		err = runGoModTidy(cli.OutDir)
		if err != nil {
			return fmt.Errorf("Failed to run go mod tidy: %w", err)
		}

		err = runGoBuild(cli.OutDir)
		if err != nil {
			return fmt.Errorf("Failed to build binary: %w", err)
		}

		if cli.Test {
			err = runTests(cli.OutDir)
			if err != nil {
				return fmt.Errorf("Test suite failed: %w", err)
			}
		}
	}

	slog.Info("Generation complete")
	return nil
}

func setLogLevel(verbosity int) {
	switch verbosity {
	case -1:
		slog.SetLogLoggerLevel(slog.LevelError) // -v=-1 (or FLASHBUILDER_LOG_LEVEL=-1)
	case 0:
		slog.SetLogLoggerLevel(slog.LevelWarn) // default
	case 1:
		slog.SetLogLoggerLevel(slog.LevelInfo) // -v
	default:
		slog.SetLogLoggerLevel(slog.LevelDebug) // -vv
	}
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
