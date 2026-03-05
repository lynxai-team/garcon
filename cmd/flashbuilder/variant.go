// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/brotli/go/cbrotli"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/vegidio/avif-go"
	"golang.org/x/sync/errgroup"
)

const (
	minSz4Brotli = 100
	maxSz4Brotli = 100 * 1024 * 1024

	minSz4AVIF = 100
	maxSz4AVIF = 20 * 1024 * 1024

	minSz4WebP = 100
	maxSz4WebP = 20 * 1024 * 1024

	// Names of the output sub-directories.
	assetsBase = "assets"
	wwwBase    = "www"
)

func ensureDirectoriesExist(cli *flags) (assetsDir, wwwDir string, useCache bool, _ error) {
	// Ensure assets/ and www/ directories are present (remains synchronous)
	assetsDir = path.Join(cli.Output, assetsBase)
	err := os.MkdirAll(assetsDir, 0o700)
	if err != nil {
		return "", "", false, fmt.Errorf("Failed to create assets directory %s: %w", assetsDir, err)
	}
	wwwDir = path.Join(cli.Output, wwwBase)
	err = os.MkdirAll(wwwDir, 0o700)
	if err != nil {
		return "", "", false, fmt.Errorf("Failed to create www directory %s: %w", wwwDir, err)
	}

	// Setup Cache
	useCache = cli.CacheMax > 0
	if useCache {
		err = os.MkdirAll(cli.CacheDir, 0o700)
		if err != nil {
			slog.Warn("disable cache", "err", err)
			useCache = false // fallback to no cache
		}
	}
	return assetsDir, wwwDir, useCache, nil
}

// linkCopyAssetsVariants links (fallback: copies) assets or their variants.
// The function generates variants for the suitable assets (depending on MIME-type and size).
// To prevent this compression or image transcoding exhausts memory on trash CPU,
// the function uses errgroup.SetLimit to strictly bound concurrency.
func linkCopyAssetsVariants(input fs.FS, assets []asset, cli *flags) error {
	assetsDir, wwwDir, useCache, err := ensureDirectoriesExist(cli)
	if err != nil {
		return err
	}

	// Initialize errgroup with context.
	g, ctx := errgroup.WithContext(context.Background())

	// Limit the concurrent goroutines: g.Go blocks the for loop
	// when the limit is reached, until a worker finishes.
	// We try to use available cores without over-spawning.
	workers := max(2, runtime.NumCPU()/2)
	g.SetLimit(workers)

	// Main processing loop.
	for i := range assets {
		a := &assets[i] // Capture pointer for mutation (goroutine)

		// Filter ineligible assets early to avoid goroutine overhead.
		if a.IsDuplicate || a.IsShortcut {
			continue
		}

		// Spawn the worker.
		// If the limit is reached, g.Go blocks until a slot is free.
		// This ensures we never have more than 'workers' goroutines active.
		g.Go(func() error {
			// Check context cancellation immediately.
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Execute the heavy work.
			return linkCopyOneAssetVariant(input, a, cli, wwwDir, assetsDir, useCache)
		})
	}

	// Wait for all active workers to finish.
	// Returns the first error (if any) and cancels the context.
	err = g.Wait()
	if err != nil {
		// Check for context cancellation specifically
		if errors.Is(err, context.Canceled) {
			slog.Warn("Processing canceled", "error", err)
		}
		return err
	}

	if useCache {
		cleanCache(cli.CacheDir, int64(cli.CacheMax))
	}

	return nil
}

// linkCopyOneAssetVariant tries to generate a variant for every suitable asset.
func linkCopyOneAssetVariant(input fs.FS, a *asset, cli *flags, wwwDir, assetsDir string, useCache bool) error { // Determine destination
	dstDir := wwwDir
	if a.IsEmbedEligible {
		dstDir = assetsDir
	}
	variantDir := dstDir
	if useCache {
		variantDir = cli.CacheDir
	}

	// Determine eligibility
	br, av, wp := variantEligibility(a.MIME)
	if br || av || wp {
		vFull, ext, size := generateOneVariant(input, a, cli, variantDir, useCache, br, av, wp)
		if vFull != "" {
			a.VariantExt = ext // update the asset in place (safe due to mutex or race-conditions handled by caller)
			a.Size = size
			if useCache {
				return linkCopyVariant(vFull, dstDir, a.Path+ext)
			}
			return nil // variant already in dstDir
		}
	}

	return linkCopyAsset(input, cli.Input, dstDir, a.Path)
}

// variantEligibility determines if content is eligible for Brotli / AVIF / WebP.
func variantEligibility(mime string) (brotliEligible, avifEligible, webpEligible bool) {
	// image/jpeg image/png image/apng image/gif image/webp image/avif image/x-icon image/vnd.microsoft.icon
	for _, suffix := range []string{"jpeg", "png", "gif", "webp", "avif", "icon"} {
		if strings.HasSuffix(mime, suffix) {
			return false, true, true
		}
	}

	// application/zip application/x-bzip application/x-bzip2 application/java-archive
	// application/gzip application/epub+zip application/x-7z-compressed font/woff2
	for _, suffix := range []string{"zip", "zip2", "compressed", "archive", "woff2"} {
		if strings.HasSuffix(mime, suffix) {
			return false, false, false
		}
	}

	// text/css text/csv text/html text/calendar text/javascript text/plain
	for _, prefix := range []string{"text"} {
		if strings.HasPrefix(mime, prefix) {
			return true, false, false
		}
	}

	// image/svg+xml application/xml application/xhtml+xml application/vnd.apple.installer+xml text/xml
	// application/manifest+json application/vnd.mozilla.xul+xml application/pdf application/ld+json
	for _, suffix := range []string{"xml", "tar", "json", "pdf"} {
		if strings.HasSuffix(mime, suffix) {
			return true, false, false
		}
	}

	return false, false, false
}

const sizeInit = math.MaxInt64

func generateOneVariant(input fs.FS, a *asset, cli *flags, variantDir string, useCache, br, av, wp bool) (vFull, ext string, size int64) {
	size = sizeInit

	if br {
		vFull, ext, size = getBrotli(input, a, cli.Brotli, variantDir, useCache)
	}

	if av {
		p, e, s := getAVIF(input, a, cli.AVIF, variantDir, useCache)
		if size > s { // keep the smallest variant
			vFull, ext, size = p, e, s
		}
	}

	if wp {
		p, e, s := getWebP(input, a, cli.WebP, variantDir, useCache)
		if size > s { // keep the smallest variant
			vFull, ext, size = p, e, s
		}
	}

	if vFull == "" {
		return "", "", 0
	}

	// Skip compression if the size reduction is too small.
	// The variant must be 7 % smaller or 3 KB smaller.
	originalSize := a.Size
	relativeLimit := originalSize * 15 / 16 // 93 % of the original
	absoluteLimit := originalSize - 3000
	minAcceptable := max(relativeLimit, absoluteLimit)
	if size > minAcceptable {
		return "", "", sizeInit // If it doesn’t beat the threshold, keep the original asset.
	}

	return vFull, ext, size
}

func enableVariant(quality int, a *asset, minSz, maxSz int64) bool {
	if quality < 0 {
		return false
	}
	if a.Size < minSz {
		slog.Debug("skip tiny", "asset", a.Path, "size", a.Size, "min", minSz)
		return false
	}
	if a.Size > maxSz {
		slog.Info("skip huge", "asset", a.Path, "size", toHuman(a.Size), "max", maxSz)
		return false
	}
	return true
}

// getBrotli retrieves Brotli from cache or generates it for asset.
func getBrotli(input fs.FS, a *asset, quality int, variantDir string, useCache bool) (vFull, _ string, size int64) {
	if !enableVariant(quality, a, minSz4Brotli, maxSz4Brotli) {
		return "", "", sizeInit
	}

	const ext = ".br"
	vFull, size = variantPath(a, variantDir, useCache, quality, ext)
	if size == 0 {
		dst := createVariantFile(vFull)
		if dst == nil {
			return "", "", sizeInit
		}
		defer dst.Close()

		slog.Info("Brotli compress", "asset", a.Path, "dst", vFull, "quality", quality)

		var err error
		size, err = compressBrotli(input, a, quality, dst)
		if err != nil {
			slog.Warn("getBrotli", "err", err)
			return "", "", sizeInit
		}
	}

	return vFull, ext, size
}

// getAVIF retrieves AVIF from cache or generates it for image asset.
// Uses github.com/vegidio/avif-go (CGO required).
func getAVIF(input fs.FS, a *asset, quality int, variantDir string, useCache bool) (vFull, _ string, size int64) {
	if !enableVariant(quality, a, minSz4AVIF, maxSz4AVIF) {
		return "", "", sizeInit
	}

	const ext = ".avif"
	vFull, size = variantPath(a, variantDir, useCache, quality, ext)
	if size == 0 {
		dst := createVariantFile(vFull)
		if dst == nil {
			return "", "", sizeInit
		}
		defer dst.Close()

		slog.Info("AVIF encode", "asset", a.Path, "dst", vFull, "quality", quality)

		var err error
		size, err = transcodeAVIF(input, a, quality, dst)
		if err != nil {
			slog.Warn("getAVIF", "err", err)
			return "", "", sizeInit
		}
	}

	return vFull, ext, size
}

// getWebP generates WebP variant for image assets
// Uses github.com/kolesa-team/go-webp/encoder (CGO required).
func getWebP(input fs.FS, a *asset, quality int, variantDir string, useCache bool) (vFull, _ string, size int64) {
	if !enableVariant(quality, a, minSz4WebP, maxSz4WebP) {
		return "", "", sizeInit
	}

	const ext = ".webp"
	vFull, size = variantPath(a, variantDir, useCache, quality, ext)
	if size == 0 {
		dst := createVariantFile(vFull)
		if dst == nil {
			return "", "", sizeInit
		}
		defer dst.Close()

		slog.Info("WebP encode", "asset", a.Path, "dst", vFull, "quality", quality)

		var err error
		size, err = transcodeWebP(input, a, quality, dst)
		if err != nil {
			slog.Warn("getWebP", "err", err)
			return "", "", sizeInit
		}
	}

	return vFull, ext, size
}

func createVariantFile(vFull string) *os.File {
	dst, err := os.Create(vFull)
	if err != nil {
		slog.Warn("createVariantFile Create", "err", err)
		return nil
	}
	return dst
}

// compressBrotli streams a file from the provided fs.FS through a Brotli
// encoder and writes the compressed output directly to path.
// It returns the number of bytes written to the destination file.
// Errors are returned to the caller; no logging, no temp‑file, no extra sync.
func compressBrotli(input fs.FS, a *asset, quality int, dst io.Writer) (int64, error) {
	// open source file: asset
	src, err := input.Open(a.Path)
	if err != nil {
		return 0, fmt.Errorf("Brotli input.Open: %w", err)
	}
	defer src.Close()

	// create Brotli writer that writes straight into dst
	enc := cbrotli.NewWriter(dst, cbrotli.WriterOptions{Quality: quality})

	// stream the data: io.Copy uses a 32 KB internal buffer
	size, err := io.Copy(enc, src)
	if err != nil {
		_ = enc.Close() // attempt graceful shutdown
		return 0, fmt.Errorf("Brotli compress copy: %w", err)
	}

	// close encoder to flush the final block
	err = enc.Close()
	if err != nil {
		return 0, fmt.Errorf("Brotli close: %w", err)
	}

	return size, nil
}

// transcodeAVIF transcodes an image asset into its AVIF variant.
// Uses github.com/vegidio/avif-go (CGO required).
func transcodeAVIF(input fs.FS, a *asset, quality int, dst *os.File) (int64, error) {
	img, err := decodeImage(input, a)
	if err != nil {
		return 0, err
	}

	quality = min(0, max(quality, 100))
	opts := &avif.Options{
		Speed:        0,       // Encoding speed, from 0-10. Higher values result in faster encoding but lower quality (default 6)
		AlphaQuality: quality, // Specifies the quality of the alpha channel (transparency), from 0-100 (default 60)
		ColorQuality: quality, // Specifies the quality of the color channels, from 0-100 (default 60)
	}

	err = avif.Encode(dst, img, opts)
	if err != nil {
		return 0, fmt.Errorf("avif.Encode %w", err)
	}

	info, err := dst.Stat()
	if err != nil {
		return 0, fmt.Errorf("encodeAVIF dst.Stat %w", err)
	}

	return info.Size(), nil
}

// transcodeWebP transcodes an image asset into its WebP variant.
// Uses github.com/kolesa-team/go-webp/encoder (CGO required).
func transcodeWebP(input fs.FS, a *asset, quality int, dst *os.File) (int64, error) {
	img, err := decodeImage(input, a)
	if err != nil {
		return 0, err
	}

	// Configure WebP options (lossy)
	opts, err := encoder.NewLossyEncoderOptions(encoder.PresetPhoto, float32(quality))
	if err != nil {
		return 0, fmt.Errorf("WebP NewLossyEncoderOptions %w", err)
	}

	enc, err := encoder.NewEncoder(img, opts)
	if err != nil {
		return 0, fmt.Errorf("WebP NewEncoder %w", err)
	}

	err = enc.Encode(dst)
	if err != nil {
		return 0, fmt.Errorf("WebP Encode %w", err)
	}

	info, err := dst.Stat()
	if err != nil {
		return 0, fmt.Errorf("encodeWebP dst.Stat %w", err)
	}

	return info.Size(), nil
}

// decodeImage decodes an image file.
func decodeImage(input fs.FS, a *asset) (image.Image, error) {
	file, err := input.Open(a.Path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open image file: %w", err)
	}
	defer file.Close()

	var img image.Image
	var decodeErr error

	// image/avif  AVIF  AV1 Image File Format
	// image/webp  WEBP  Web Picture format
	// image/gif   GIF   Graphics Interchange Format
	// image/jpeg  JPEG  Joint Photographic Expert Group
	// image/png   PNG   Portable Network Graphics
	// image/apng  APNG  Animated PNG
	// image/x-icon  favicon.ico
	// image/vnd.microsoft.icon

	switch a.MIME {
	case "image/jpeg":
		img, decodeErr = jpeg.Decode(file) // jpeg.Decode reads full content
	case "image/png":
		img, decodeErr = png.Decode(file)
	case "image/gif":
		img, decodeErr = gif.Decode(file)
	default:
		img, _, decodeErr = image.Decode(file)
	}
	if decodeErr != nil {
		return nil, fmt.Errorf("Decode %s (%s) %s: %w", a.Path, toHuman(a.Size), a.MIME, decodeErr)
	}

	return img, nil
}

func variantPath(a *asset, dir string, useCache bool, quality int, ext string) (string, int64) {
	vFull := a.Path + ext // in the assets/ or www/ directory
	if useCache {
		vFull = strconv.Itoa(quality) + a.ETag + ext // in the cache directory
	}

	vFull = path.Join(dir, vFull)
	info, err := os.Stat(vFull)
	if err != nil {
		return vFull, 0 // variant does not yet exist => generate it
	}

	size := info.Size()
	if size == 0 {
		os.Remove(vFull)
		return vFull, 0
	}

	// variant exists => reuse it
	slog.Debug("reuse variant", "asset", a.Path, "sizeA", toHuman(a.Size), "sizeV", toHuman(size), "variant", vFull)
	return vFull, size
}

// cleanCache maintains cache size within configured limits
// - removes oldest files when cache exceeds maxSize.
// - removes empty files.
func cleanCache(cacheDir string, maxSize int64) {
	type fileInfo struct {
		modTime time.Time
		path    string
		size    int64
	}
	var files []fileInfo
	var total int64

	err := filepath.WalkDir(cacheDir, func(variantPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		info, walkErr := entry.Info()
		if walkErr != nil {
			return nil
		}
		if info.Size() == 0 {
			os.Remove(variantPath)
			return nil
		}
		files = append(files, fileInfo{
			path:    variantPath,
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		total += info.Size()
		return nil
	})
	if err != nil {
		slog.Warn("Failed to walk cache directory", "err", err)
		return
	}

	// If total size exceeds max, delete oldest files
	if total > maxSize {
		// Sort by modification time (oldest first)
		sort.Slice(files, func(i, j int) bool {
			return files[i].modTime.Before(files[j].modTime)
		})

		// Delete oldest files until total size is under max
		for _, file := range files {
			if total <= maxSize {
				break
			}
			os.Remove(file.path)
			total -= file.size
		}
	}
}

// allocateEmbedBudget determines which assets are eligible for embedding
// Assets are sorted by size (smallest first) and embedded until budget exhausted.
func allocateEmbedBudget(assets []asset, budget int64) {
	// Sort assets by size (smallest first for embedding priority)
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Size < assets[j].Size
	})

	var total int64
	for i := range assets {
		if total+assets[i].Size > budget {
			break
		}
		assets[i].IsEmbedEligible = true
		total += assets[i].Size
	}
}

func linkCopyAsset(input fs.FS, inputDir, dstDir, assetPath string) error {
	srcFull := path.Join(inputDir, assetPath)
	dstFull := path.Join(dstDir, assetPath)

	dstFullDir := path.Dir(dstFull)
	if dstFullDir != dstDir { // dstDir already exists => avoid os.MkdirAll(dstDir)
		err := os.MkdirAll(dstFullDir, 0o700)
		if err != nil {
			return fmt.Errorf("linkCopyAsset MkdirAll: %w", err)
		}
	}

	os.Remove(dstFull)               // Remove existing if present
	err := os.Link(srcFull, dstFull) // Create hard-link
	if err == nil {
		slog.Debug("hard-link", "asset", srcFull, "dst", dstFull)
	} else {
		return copyAsset(input, assetPath, dstFull) // fallback: copy
	}
	return nil
}

func linkCopyVariant(vCacheFull, dstDir, dstPath string) error {
	dstFull := path.Join(dstDir, dstPath)

	dstFullDir := path.Dir(dstFull)
	if dstFullDir != dstDir { // dstDir already exists => avoid os.MkdirAll(dstDir)
		err := os.MkdirAll(dstFullDir, 0o700)
		if err != nil {
			return fmt.Errorf("linkCopyVariant MkdirAll: %w", err)
		}
	}

	os.Remove(dstFull)                  // Remove existing if present
	err := os.Link(vCacheFull, dstFull) // Create hard-link
	if err == nil {
		slog.Debug("hard-link", "variant", vCacheFull, "dst", dstFull)
	} else {
		return copyVariant(vCacheFull, dstFull) // fallback: copy
	}
	return nil
}

// copyAsset copies the source file to the destination, overwriting the destination if necessary.
func copyAsset(srcFS fs.FS, srcPath, dstFull string) error {
	src, err := srcFS.Open(srcPath)
	if err != nil {
		return fmt.Errorf("copyAsset Open: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(dstFull)
	if err != nil {
		return fmt.Errorf("copyAsset Create: %w", err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("copyAsset Copy: %w", err)
	}

	slog.Debug("fallback copy", "asset", srcPath, "dst", dstFull)
	return nil
}

// copyVariant copies the source file to the destination, overwriting the destination if necessary.
func copyVariant(srcFull, dstFull string) error {
	src, err := os.Open(srcFull)
	if err != nil {
		return fmt.Errorf("copyVariant Open %q: %w", srcFull, err)
	}
	defer src.Close()

	dst, err := os.Create(dstFull)
	if err != nil {
		return fmt.Errorf("copyVariant Create: %w", err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("copyVariant Copy: %w", err)
	}

	slog.Debug("fallback copy", "variant", srcFull, "dst", dstFull)
	return nil
}
