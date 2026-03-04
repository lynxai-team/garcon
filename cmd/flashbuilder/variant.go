// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
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

// copyAssetsAndVariants links (or copies) assets or their variants.
func copyAssetsAndVariants(input fs.ReadFileFS, assets []asset, cli *flags) error {
	// create assets directory
	assetsDir := path.Join(cli.Output, assetsBase)
	err := os.MkdirAll(assetsDir, 0o700)
	if err != nil {
		return fmt.Errorf("Failed to create assets directory %s: %w", assetsDir, err)
	}

	// create www directory
	wwwDir := path.Join(cli.Output, wwwBase)
	err = os.MkdirAll(wwwDir, 0o700)
	if err != nil {
		return fmt.Errorf("Failed to create www directory %s: %w", wwwDir, err)
	}

	useCache := true
	if cli.CacheMax == 0 { // disables cache
		useCache = false
		slog.Info("disable cache", "CacheMax", cli.CacheMax)
	} else {
		// ensure cache directory exists
		err = os.MkdirAll(cli.CacheDir, 0o700)
		if err != nil {
			useCache = false
			slog.Warn("disable cache", "err", err)
		}
	}

	// use errgroup for concurrency
	g, _ := errgroup.WithContext(context.Background())

	for i := range assets {
		if assets[i].IsDuplicate {
			continue
		}

		br, av, wp := variantEligibility(assets[i].MIME)
		generate := br || av || wp
		if !generate {
			continue
		}

		dstDir := wwwDir
		if assets[i].IsEmbedEligible {
			dstDir = assetsDir
		}

		variantDir := dstDir
		if useCache {
			variantDir = cli.CacheDir
		}

		g.Go(func() error {
			vFull, ext, size := generateOneVariant(input, &assets[i], cli, variantDir, useCache, br, av, wp)
			if size > 0 {
				assets[i].VariantExt = ext
				assets[i].Size = size
				if useCache {
					return linkCopyVariant(vFull, dstDir, assets[i].Path+ext)
				}
				return nil // variant already in the dstDir
			}
			return linkCopyAsset(input, cli.Input, dstDir, assets[i].Path)
		})
	}

	err = g.Wait()
	if err != nil {
		return err
	}

	if useCache {
		cleanCache(cli.CacheDir, int64(cli.CacheMax))
	}

	return nil
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

func generateOneVariant(input fs.ReadFileFS, a *asset, cli *flags, variantDir string, useCache, br, av, wp bool) (vFull, ext string, size int64) {
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

func enableVariant(quality int, aSize, minSz, maxSz int64) bool {
	if quality < 0 {
		return false
	}
	if aSize < minSz {
		slog.Debug("skip tiny asset", "size", aSize, "min", minSz)
		return false
	}
	if aSize > maxSz {
		slog.Info("skip huge asset", "size", aSize, "max", maxSz)
		return false
	}
	return true
}

// getBrotli retrieves Brotli from cache or generates it for asset.
func getBrotli(input fs.ReadFileFS, a *asset, quality int, variantDir string, useCache bool) (vFull, _ string, size int64) {
	if !enableVariant(quality, a.Size, minSz4Brotli, maxSz4Brotli) {
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
func getAVIF(input fs.ReadFileFS, a *asset, quality int, variantDir string, useCache bool) (vFull, _ string, size int64) {
	if !enableVariant(quality, a.Size, minSz4AVIF, maxSz4AVIF) {
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
		size, err = encodeAVIF(input, a, quality, dst)
		if err != nil {
			slog.Warn("getAVIF", "err", err)
			return "", "", sizeInit
		}
	}

	return vFull, ext, size
}

// getWebP generates WebP variant for image assets
// Uses github.com/kolesa-team/go-webp/encoder (CGO required).
func getWebP(input fs.ReadFileFS, a *asset, quality int, variantDir string, useCache bool) (vFull, _ string, size int64) {
	if !enableVariant(quality, a.Size, minSz4WebP, maxSz4WebP) {
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
		size, err = encodeWebP(input, a, quality, dst)
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
func compressBrotli(input fs.ReadFileFS, a *asset, quality int, dst io.Writer) (int64, error) {
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

// encodeAVIF generates AVIF for an image asset and writes it in cacheDir
// Uses github.com/vegidio/avif-go (CGO required).
func encodeAVIF(input fs.ReadFileFS, a *asset, quality int, dst *os.File) (int64, error) {
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

// encodeWebP generates WebP variant for image assets
// Uses github.com/kolesa-team/go-webp/encoder (CGO required).
func encodeWebP(input fs.ReadFileFS, a *asset, quality int, dst *os.File) (int64, error) {
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
func decodeImage(input fs.ReadFileFS, a *asset) (image.Image, error) {
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
		return nil, fmt.Errorf("Failed to decode image %s (%s): %w", a.Path, toHuman(a.Size), decodeErr)
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
	slog.Debug("reuse", "variant", vFull, "size", toHuman(size))
	return vFull, size
}

// cleanCache maintains cache size within configured limits
// Removes oldest files when cache exceeds maxSize.
func cleanCache(cacheDir string, maxSize int64) {
	type fileInfo struct {
		modTime time.Time
		path    string
		size    int64
	}
	var files []fileInfo
	var total int64

	err := filepath.WalkDir(cacheDir, func(vFull string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		info, walkErr := entry.Info()
		if walkErr != nil {
			return nil
		}
		files = append(files, fileInfo{
			path:    vFull,
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

// allocateBudget determines which assets are eligible for embedding
// Assets are sorted by size (smallest first) and embedded until budget exhausted.
func allocateBudget(assets []asset, budget int64) []asset {
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

	return assets
}

func linkCopyAsset(input fs.ReadFileFS, inputDir, dstDir, assetPath string) error {
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
		return fmt.Errorf("copyVariant Open: %w", err)
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
