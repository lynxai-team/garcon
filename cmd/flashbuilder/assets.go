// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/kalafut/imohash"
	"github.com/mtraver/base91"
)

// asset represents a static asset with all pre-computed metadata.
type asset struct {
	CanonicalID    string
	AbsPath        string
	Filename       string
	MIME           string
	RelPath        string
	ETag           string
	Identifier     string
	ImoHash        []byte
	Variants       []Variant
	Size           int64
	FrequencyScore int
	EmbedEligible  bool
	IsDuplicate    bool
	IsShortcut     bool
	IsIndex        bool
	IsHTML         bool
}

// Variant represents a compression variant for an asset.
type Variant struct {
	Identifier  string
	Extension   string
	CachePath   string
	HeaderHTTP  []byte
	HeaderHTTPS []byte
	VariantType VariantType
	Size        int64
}

// VariantType represents compression type.
type VariantType int

const (
	VariantBrotli VariantType = iota
	VariantAVIF
	VariantWebP
)

// discover walks the input directory and collects all files
// Returns assets sorted by relative path for deterministic ordering.
func discover(input string) ([]asset, error) {
	var assets []asset

	err := filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("E001: Failed to access path %s: %w", path, err)
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip special files (sockets, devices, named pipes)
		mode := info.Mode()
		if mode&os.ModeSocket != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 {
			return nil
		}

		// Follow symlinks to resolve actual file
		resolvedPath := path
		if mode&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err == nil {
				absTarget := filepath.Join(filepath.Dir(path), target)
				targetInfo, err := os.Stat(absTarget)
				if err == nil && !targetInfo.IsDir() {
					resolvedPath = absTarget
					info = targetInfo
				}
			}
		}

		// Compute relative path (POSIX-style with forward slashes)
		relPath, err := filepath.Rel(input, resolvedPath)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Compute absolute path
		absPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			return nil
		}

		// Detect MIME type
		mimeType := detectMIME(resolvedPath)

		// Determine if index file
		isIndex := strings.HasSuffix(relPath, "index.html") || relPath == "index.html"

		// Determine if HTML content
		isHTML := strings.HasPrefix(mimeType, "text/html") ||
			strings.HasPrefix(mimeType, "application/xhtml+xml")

		a := asset{
			RelPath: relPath,
			AbsPath: absPath,
			Size:    info.Size(),
			MIME:    mimeType,
			IsIndex: isIndex,
			IsHTML:  isHTML,
		}
		assets = append(assets, a)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by relative path for deterministic ordering
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].RelPath < assets[j].RelPath
	})

	return assets, nil
}

// detectMIME determines the MIME type for a file
// Step 1: Extension-based lookup
// Step 2: Content sniffing
// Step 3: Fallback to application/octet-stream.
func detectMIME(path string) string {
	// Step 1: Extension-based lookup
	ext := filepath.Ext(path)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType
		}
	}

	// Step 2: Content sniffing (first 512 bytes)
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		sniff := data
		if len(sniff) > 512 {
			sniff = sniff[:512]
		}
		mimeType := http.DetectContentType(sniff)
		if mimeType != "" {
			return mimeType
		}
	}

	// Step 3: Fallback
	return "application/octet-stream"
}

// dedupe identifies duplicate assets based on content hash
// First asset with identical hash is canonical, others are duplicates.
func dedupe(assets []asset) []asset {
	hashMap := make(map[string][]int) // Map hash -> slice of asset indices

	// Group assets by hash
	for i, a := range assets {
		key := string(a.ImoHash)
		hashMap[key] = append(hashMap[key], i)
	}

	// Mark duplicates
	for _, indices := range hashMap {
		if len(indices) > 1 {
			// First asset is canonical
			canonicalIdx := indices[0]
			canonical := assets[canonicalIdx]

			// Verify canonical content exists
			canonicalContent, err := os.ReadFile(canonical.AbsPath)
			if err != nil {
				continue
			}

			// Mark remaining assets as duplicates
			for i := 1; i < len(indices); i++ {
				idx := indices[i]
				duplicateContent, err := os.ReadFile(assets[idx].AbsPath)
				if err == nil && string(canonicalContent) == string(duplicateContent) {
					assets[idx].IsDuplicate = true
					assets[idx].CanonicalID = canonical.Identifier
				}
			}
		}
	}

	return assets
}

// generateIdentifier creates a valid Go identifier from a relative path
// Deterministic sanitization ensures reproducibility.
func generateIdentifier(relPath string, existing map[string]bool) string {
	// Split path into segments
	segments := strings.Split(relPath, "/")
	filename := filepath.Base(relPath)
	ext := filepath.Ext(filename)
	filename = strings.TrimSuffix(filename, ext)

	// Filter and capitalize each segment
	var parts []string
	for _, seg := range segments {
		seg = sanitizeIdentifier(seg)
		if len(seg) > 0 {
			parts = append(parts, capitalize(seg))
		}
	}

	// Add filename if present
	if filename != "" && filename != "." {
		filename = sanitizeIdentifier(filename)
		if len(filename) > 0 {
			parts = append(parts, capitalize(filename))
		}
	}

	identifier := "Asset" + strings.Join(parts, "")

	// Resolve collisions with numeric suffix
	if existing[identifier] {
		for i := 2; i < 1000; i++ {
			newID := fmt.Sprintf("%s_%03d", identifier, i)
			if !existing[newID] {
				identifier = newID
				break
			}
		}
	}

	return identifier
}

// sanitizeIdentifier filters valid Go identifier characters
// Exported for use in template functions.
func sanitizeIdentifier(s string) string {
	var result strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// capitalize uppercases the first character.
func capitalize(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// estimateFrequencyScore estimates request frequency for switch case ordering
// Higher frequency assets appear first in switch statements for better branch prediction.
func estimateFrequencyScore(path string, isEmbed bool) int {
	score := 0

	if path == "" || path == "index.html" {
		score += 1000
	}
	if strings.Contains(path, "favicon.") {
		score += 900
	}
	if strings.HasSuffix(path, ".css") {
		score += 800
	}
	if strings.HasSuffix(path, ".js") {
		score += 600
	}
	if strings.Contains(path, "index.html") {
		score += 500
	}
	if strings.Contains(path, "logo.") {
		score += 400
	}
	if isEmbed {
		score += 200
	}

	// Path complexity penalty
	score -= 5 * len(path)
	score -= 30 * strings.Count(path, "/")

	// Low-traffic extensions penalty
	lowTraffic := []string{".map", ".zip", ".pdf", ".doc", ".xls", ".tar"}
	for _, ext := range lowTraffic {
		if strings.HasSuffix(path, ext) {
			score -= 100
			break
		}
	}

	return score
}

// generateShortcut creates an extensionless shortcut for a path
// Enables clean URLs like "/about" instead of "/about/index.html".
func generateShortcut(relPath string) string {
	// Root index has no shortcut
	if relPath == "index.html" {
		return ""
	}

	// Index files in subdirectories
	if before, ok := strings.CutSuffix(relPath, "/index.html"); ok {
		return before
	}

	// Extensionless shortcuts
	ext := filepath.Ext(relPath)
	if ext != "" {
		return strings.TrimSuffix(relPath, ext)
	}

	return relPath
}

// createLinks creates symbolic links for assets in the output directory.
func createLinks(assets []asset, input, output, cacheDir string) error {
	// Create assets directory
	assetsDir := filepath.Join(output, "assets")
	err := os.MkdirAll(assetsDir, 0o755)
	if err != nil {
		return fmt.Errorf("E087: Failed to create assets directory: %w", err)
	}

	// Create www directory
	wwwDir := filepath.Join(output, "www")
	err = os.MkdirAll(wwwDir, 0o755)
	if err != nil {
		return fmt.Errorf("E087: Failed to create www directory: %w", err)
	}

	for _, asset := range assets {
		if asset.IsDuplicate {
			continue
		}
		if asset.IsShortcut {
			continue
		}

		if asset.EmbedEligible {
			// Create symlink in assets directory
			target := filepath.Join(assetsDir, asset.Filename+filepath.Ext(asset.RelPath))
			// Remove existing file/link if present
			os.Remove(target)
			err := os.Symlink(asset.AbsPath, target)
			if err != nil {
				return fmt.Errorf("E087: Failed to create symlink: %w", err)
			}
		} else {
			// Create symlink in www directory
			target := filepath.Join(wwwDir, asset.RelPath)
			err := os.MkdirAll(filepath.Dir(target), 0o755)
			if err != nil {
				return fmt.Errorf("E087: Failed to create www subdirectory: %w", err)
			}
			// Remove existing file/link if present
			os.Remove(target)
			err = os.Symlink(asset.AbsPath, target)
			if err != nil {
				return fmt.Errorf("E087: Failed to create symlink: %w", err)
			}
		}
	}

	return nil
}

// computeImoHash computes the ImoHash for a file (128 bits)
// Uses github.com/kalafut/imohash.
func computeImoHash(path string) []byte {
	sum, err := imohash.SumFile(path)
	if err != nil {
		return nil
	}
	return sum[:] // 128 bits
}

// computeETag generates an ETag from ImoHash
// Uses base91 encoding for compact representation.
func computeETag(hash []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&()*+,./:;<=>?@[]^_ {|}~'"
	encoder := base91.NewEncoding(alphabet)
	b91 := encoder.EncodeToString(hash) // the base91-encoded hash is 20 bytes
	return b91[:9]                      // truncate 9 bytes out of 20
}
