// Package: main
// Purpose: CLI, entry point, cli struct, orchestration
// File: main.go

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/alecthomas/units"
)

// CLI structure
type cli struct {
	Input       string           `env:"FLASHBUILDER_INPUT" type:"path" arg:"input"  help:"Path to asset tree"`
	Output      string           `env:"FLASHBUILDER_OUTPUT" type:"path" arg:"output" help:"Destination for generated files"`
	EmbedBudget units.Base2Bytes `env:"FLASHBUILDER_EMBED_BUDGET" default:"200GB"`
	Brotli      int              `env:"FLASHBUILDER_BROTLI" default:"11"`
	AVIF        int              `env:"FLASHBUILDER_AVIF" default:"50"`
	WebP        int              `env:"FLASHBUILDER_WEBP" default:"50"`
	CSP         string           `env:"FLASHBUILDER_CSP" default:"default-src 'self'"`
	Verbosity   int              `env:"FLASHBUILDER_LOG_LEVEL" type:"counter" short:"v"`
	DryRun      bool             `env:"FLASHBUILDER_DRY_RUN"`
	Tests       bool             `env:"FLASHBUILDER_TESTS"`
	CacheMax    units.Base2Bytes `env:"FLASHBUILDER_CACHE_MAX" default:"5GB"`
	CacheDir    string           `env:"FLASHBUILDER_CACHE_DIR"`
}

func main() {
	// Kong handles --help flag and environment variables automatically via tags
	var cli cli
	kong.Parse(&cli)

	// Validate inputs
	absInput, _ := filepath.Abs(cli.Input)
	absOutput, _ := filepath.Abs(cli.Output)
	if absInput == absOutput {
		log.Println("E099: Input and output must differ")
		os.Exit(2)
	}

	// Validate compression flags
	if err := validateCompressionFlags(&cli); err != nil {
		log.Println(err.Error())
		os.Exit(2)
	}

	// Set default cache directory
	if cli.CacheDir == "" {
		cli.CacheDir = getDefaultCacheDir()
	}

	// Parse size flags using units.Base2Bytes
	embedBudget := int64(cli.EmbedBudget)
	cacheMax := int64(cli.CacheMax)

	// Ensure cache directory exists and clean cache if needed
	if !cli.DryRun {
		if err := ensureCacheDir(cli.CacheDir); err != nil {
			log.Printf("E099: Failed to create cache directory: %v", err)
			os.Exit(2)
		}
		// Clean cache to maintain size limit
		if err := cleanCache(cli.CacheDir, cacheMax); err != nil {
			log.Printf("E099: Failed to clean cache: %v", err)
			os.Exit(2)
		}
	}

	// Create output directories
	if !cli.DryRun {
		os.MkdirAll(cli.Output, 0755)
		os.MkdirAll(filepath.Join(cli.Output, "assets"), 0755)
		os.MkdirAll(filepath.Join(cli.Output, "www"), 0755)
	}

	// Step 1: Discover assets
	assets, err := discover(cli.Input)
	if err != nil {
		log.Printf("E001: Failed to discover assets: %v", err)
		os.Exit(2)
	}

	// Step 2: Compute hashes and ETags
	for i := range assets {
		assets[i].ImoHash = computeImoHash(assets[i].AbsPath)
		assets[i].ETag = computeETag(assets[i].ImoHash)
	}

	// Step 3: Deduplicate
	assets = dedupe(assets)

	// Step 4: Generate identifiers
	identifiers := make(map[string]bool)
	for i := range assets {
		assets[i].Identifier = generateIdentifier(assets[i].RelPath, identifiers)
		identifiers[assets[i].Identifier] = true
		assets[i].Filename = assets[i].Identifier + filepath.Ext(assets[i].RelPath)
	}

	// Step 5: Generate variants
	assets = generateVariants(assets, cli.Brotli, cli.AVIF, cli.WebP, cli.CacheDir)

	// Step 6: Allocate embed budget
	assets = allocateBudget(assets, embedBudget)

	// Step 7: Update frequency scores
	for i := range assets {
		assets[i].FrequencyScore = estimateFrequencyScore(assets[i].RelPath, assets[i].EmbedEligible)
	}

	// Step 8: Pre-compute headers
	for i := range assets {
		if assets[i].EmbedEligible && !assets[i].IsDuplicate {
			assets[i].HeaderHTTP = renderHeaderHTTP(assets[i], cli.CSP)
			assets[i].HeaderHTTPS = renderHeaderHTTPS(assets[i], cli.CSP, "8443")
		}
	}

	// Step 9: Create links
	if !cli.DryRun {
		if err := createLinks(assets, cli.Input, cli.Output, cli.CacheDir); err != nil {
			log.Printf("E087: Failed to create links: %v", err)
			os.Exit(87)
		}
	}

	// Step 10: Add shortcuts
	assets = addShortcuts(assets)

	// Step 11: Compute MaxLen
	maxLen := computeMaxLen(assets)

	// Step 12: Generate dispatch arrays
	dispatch := buildDispatch(assets, maxLen)

	// Step 13: Convert to template data
	data := TemplateData{
		Config: ConfigData{
			CSP:       cli.CSP,
			HTTPSPort: "8443",
			Module:    "flash",
		},
		Assets:   convertAssets(assets),
		Dispatch: dispatch,
		MaxLen:   maxLen,
	}

	// Step 14: Generate Go code
	err = generate(data, cli.Output, cli.DryRun)
	if err != nil {
		log.Printf("E099: Failed to generate code: %v", err)
		os.Exit(2)
	}

	if !cli.DryRun {
		// Step 15: Run go mod tidy
		if err := runGoModTidy(cli.Output); err != nil {
			log.Printf("E099: Failed to run go mod tidy: %v", err)
			os.Exit(2)
		}

		// Step 16: Build binary
		if err := runGoBuild(cli.Output); err != nil {
			log.Printf("E099: Failed to build binary: %v", err)
			os.Exit(2)
		}

		// Step 17: Run tests
		if err := runTests(cli.Output); err != nil {
			log.Printf("E079: Test suite failed: %v", err)
			os.Exit(3)
		}
	}

	fmt.Println("Generation complete")
}

// getDefaultCacheDir returns the default cache directory
// Follows XDG Base Directory Specification
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

// validateCompressionFlags validates compression quality flags
func validateCompressionFlags(cli *cli) error {
	// Brotli quality: 0-11
	if cli.Brotli < 0 || cli.Brotli > 11 {
		return fmt.Errorf("E025: Brotli quality must be 0-11, got %d", cli.Brotli)
	}

	// AVIF quality: 0-100
	if cli.AVIF < 0 || cli.AVIF > 100 {
		return fmt.Errorf("E025: AVIF quality must be 0-100, got %d", cli.AVIF)
	}

	// WebP quality: 0-100
	if cli.WebP < 0 || cli.WebP > 100 {
		return fmt.Errorf("E025: WebP quality must be 0-100, got %d", cli.WebP)
	}

	return nil
}

// runGoModTidy runs go mod tidy in the output directory
func runGoModTidy(output string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = output
	return cmd.Run()
}

// runGoBuild builds the flash binary
func runGoBuild(output string) error {
	cmd := exec.Command("go", "build", "-o", "flash")
	cmd.Dir = output
	return cmd.Run()
}

// runTests runs the test suite
func runTests(output string) error {
	cmd := exec.Command("go", "test", "-v", "-race", "-vet")
	cmd.Dir = output
	return cmd.Run()
}
