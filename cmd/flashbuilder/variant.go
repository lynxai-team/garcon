// Package: main
// Purpose: Variant generation, compression
// File: variant.go

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/brotli/go/cbrotli"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/vegidio/avif-go"
)

// generateVariants creates compression variants for eligible assets
// Only generates variants if compressed size is smaller than original
func generateVariants(assets []asset, brotliQuality, avifQuality, webPQuality int, cacheDir string) []asset {
	for i := range assets {
		if !assets[i].EmbedEligible {
			continue
		}

		var variants []Variant

		// Generate Brotli variant for compressible MIME types
		if isCompressible(assets[i].MIME) && brotliQuality > 0 {
			content, err := os.ReadFile(assets[i].AbsPath)
			if err == nil {
				compressed, err := compressBrotli(content, brotliQuality)
				if err == nil && int64(len(compressed)) < assets[i].Size {
					v := Variant{
						VariantType: VariantBrotli,
						Size:        int64(len(compressed)),
						Identifier:  assets[i].Identifier + "_brotli",
						Extension:   ".br",
						CachePath:   filepath.Join(cacheDir, assets[i].RelPath+".br"),
					}
					if err := os.WriteFile(v.CachePath, compressed, 0644); err == nil {
						variants = append(variants, v)
					}
				}
			}
		}

		// Generate AVIF variant for images
		if isImage(assets[i].MIME) && avifQuality > 0 {
			v, err := generateAVIFVariant(assets[i], avifQuality, cacheDir)
			if err == nil && v.Size < assets[i].Size {
				variants = append(variants, v)
			}
		}

		// Generate WebP variant for images
		if isImage(assets[i].MIME) && webPQuality > 0 {
			v, err := generateWebPVariant(assets[i], webPQuality, cacheDir)
			if err == nil && v.Size < assets[i].Size {
				variants = append(variants, v)
			}
		}

		assets[i].Variants = variants
	}

	return assets
}

// isCompressible determines if content is eligible for Brotli compression
func isCompressible(mime string) bool {
	compressible := []string{
		"text/html", "text/css", "text/javascript", "application/javascript",
		"text/plain", "text/xml", "application/json", "application/xml",
	}
	for _, m := range compressible {
		if strings.HasPrefix(mime, m) {
			return true
		}
	}
	return false
}

// isImage determines if content is an image
func isImage(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

// compressBrotli compresses content using Brotli
// Uses github.com/google/brotli/go/cbrotli
func compressBrotli(content []byte, quality int) ([]byte, error) {
	opts := cbrotli.WriterOptions{
		Quality: quality,
		LGWin:   0, // Automatic window size
	}
	return cbrotli.Encode(content, opts)
}

// generateAVIFVariant generates AVIF variant for image assets
// Uses github.com/vegidio/avif-go (CGO required)
func generateAVIFVariant(asset asset, quality int, cacheDir string) (Variant, error) {
	img, err := decodeImage(asset.AbsPath)
	if err != nil {
		return Variant{}, err
	}

	opts := &avif.Options{
		Speed:        6,
		AlphaQuality: quality,
		ColorQuality: quality,
	}

	var buf bytes.Buffer
	err = avif.Encode(&buf, img, opts)
	if err != nil {
		return Variant{}, err
	}

	v := Variant{
		VariantType: VariantAVIF,
		Size:        int64(buf.Len()),
		Identifier:  asset.Identifier + "_avif",
		Extension:   ".avif",
		CachePath:   filepath.Join(cacheDir, asset.RelPath+".avif"),
	}

	if err := os.WriteFile(v.CachePath, buf.Bytes(), 0644); err != nil {
		return Variant{}, err
	}

	return v, nil
}

// generateWebPVariant generates WebP variant for image assets
// Uses github.com/kolesa-team/go-webp/encoder (CGO required)
func generateWebPVariant(asset asset, quality int, cacheDir string) (Variant, error) {
	img, err := decodeImage(asset.AbsPath)
	if err != nil {
		return Variant{}, err
	}

	// Configure WebP options (lossy)
	opts, err := encoder.NewLossyEncoderOptions(encoder.PresetPhoto, float32(quality))
	if err != nil {
		return Variant{}, err
	}

	enc, err := encoder.NewEncoder(img, opts)
	if err != nil {
		return Variant{}, err
	}

	var buf bytes.Buffer
	err = enc.Encode(&buf)
	if err != nil {
		return Variant{}, err
	}

	v := Variant{
		VariantType: VariantWebP,
		Size:        int64(buf.Len()),
		Identifier:  asset.Identifier + "_webp",
		Extension:   ".webp",
		CachePath:   filepath.Join(cacheDir, asset.RelPath+".webp"),
	}

	if err := os.WriteFile(v.CachePath, buf.Bytes(), 0644); err != nil {
		return Variant{}, err
	}

	return v, nil
}

// decodeImage decodes an image file using standard library decoders
func decodeImage(absPath string) (image.Image, error) {
	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("E001: Failed to open image file: %v", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(absPath))

	var img image.Image
	var decodeErr error

	switch ext {
	case ".jpg", ".jpeg":
		img, decodeErr = jpeg.Decode(file)
	case ".png":
		img, decodeErr = png.Decode(file)
	case ".gif":
		img, decodeErr = gif.Decode(file)
	default:
		img, _, decodeErr = image.Decode(file)
	}

	if decodeErr != nil {
		return nil, fmt.Errorf("E001: Failed to decode image: %v", decodeErr)
	}

	return img, nil
}
