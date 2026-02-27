// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/kalafut/imohash"
	"github.com/mtraver/base91"
)

// asset represents a static asset with all pre-computed metadata.
type asset struct {
	Identifier string // Go identifier (e.g., "AssetCSS")
	Filename   string // Filename in assets/ directory
	RelPath    string // POSIX-style relative path (forward slashes)
	AbsPath    string // Absolute path to source file
	MIME       string // Detected MIME type (e.g., "text/html")
	Size       int64  // File size in bytes

	// headers
	CSP      string    // Content-Security-Policy header value
	ETag     string    // Base91 ETag for conditional GET (quoted)
	ImoHash  uint128   // Content hash from imohash
	Variants []variant // Compression variants (Brotli, AVIF, WebP)

	Frequency int // Request frequency score for switch ordering

	EmbedEligible bool // Selected for embedding within budget
	IsDuplicate   bool // Content matches another asset
	IsShortcut    bool

	// HTML
	IsHTML  bool // Is HTML content (for CSP injection)
	IsIndex bool // Is index file (e.g., index.html)
}

// variant represents a compression variant for an asset.
type variant struct {
	Identifier  string      // Go identifier for this variant
	Extension   string      // File extension (e.g., ".br", ".avif", ".webp")
	CachePath   string      // Cache location for this variant
	HeaderHTTP  []byte      // HTTP headers for this variant
	HeaderHTTPS []byte      // HTTPS headers for this variant
	Size        int64       // Variant size in bytes
	VariantType variantType // Compression type (Brotli, AVIF, WebP)
}

// variantType represents compression type.
type variantType int

const (
	VariantBrotli variantType = iota
	VariantAVIF
	VariantWebP
)

// discover walks the input directory and collects all files
// Returns assets sorted by relative path for deterministic ordering.
func discover(input, csp string) ([]asset, error) {
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

		c := ""
		if isHTML {
			html := read(absPath, 100_000) // limit 100 KB
			c = extractCSP(html)
			if c == "" {
				c = csp
			}
		}

		a := asset{
			AbsPath: absPath,
			RelPath: relPath,
			Size:    info.Size(),
			MIME:    mimeType,

			// only HTML:
			IsHTML:  isHTML,
			IsIndex: isIndex,
			CSP:     c,    // Content-Security-Policy (HTTP header)
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

func read(filename string, limit int) []byte {
	f, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer f.Close()

	firstBytes := make([]byte, 0, limit)
	n, err := f.Read(firstBytes)
	return firstBytes[:n]
}

// extractCSP extracts Content-Security-Policy from HTML. Priority:
// 1. If HTML contains `<meta http-equiv="Content-Security-Policy" content="...">`, use extracted value
// 2. Else if `--csp=""` is explicitly set, no CSP header
// 3. Else use `--csp` default value (`"default-src 'self'"`).
func extractCSP(html []byte) string {
	csp := extract(html,
		[]byte(`meta http-equiv="content-security-policy"`),
		[]byte(` content="`))
	return string(csp)
}
func extract(html, tag, field []byte) []byte {
	lower := bytes.ToLower(html)
	tagIdx := bytes.Index(lower, tag)
	if tagIdx == -1 {
		return nil
	}
	tagIdx += len(tag)

	fieldIdx := bytes.Index(lower[tagIdx:], field)
	if fieldIdx == -1 {
		return nil
	}
	fieldIdx += len(field)

	start := tagIdx + fieldIdx
	end := bytes.IndexByte(lower[start:], '"')
	if end == -1 {
		return nil
	}

	return html[start : start+end]
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

// deduplicate identifies duplicate assets based on content hash.
func deduplicate(assets []asset) []asset {
	hashMap := make(map[uint128][]int) // Map hash -> slice of asset indices

	// Use the shorter path as the canonical asset: Sort by path length
	// First asset with identical hash is canonical, others are duplicates.
	sort.Slice(assets, func(i, j int) bool {
		return len(assets[i].RelPath) < len(assets[j].RelPath)
	})

	// Group assets by hash
	for i, a := range assets {
		key := a.ImoHash
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
					assets[idx].Identifier = canonical.Identifier
				}
			}
		}
	}

	return assets
}

func setIdentifiers(assets []asset) []asset {
	identifiers := make(map[string]struct{}, len(assets))
	filenames := make(map[string]struct{}, len(assets))
	for i := range assets {
		id, fn := generateIdentifier(assets[i].RelPath, identifiers, filenames)
		identifiers[id] = struct{}{}
		filenames[fn] = struct{}{}
		assets[i].Identifier = id
		assets[i].Filename = fn
	}
	return assets
}

// generateIdentifier creates a valid Go identifier from a relative path
// Deterministic sanitization ensures reproducibility.
func generateIdentifier(relPath string, identifiers, filenames map[string]struct{}) (id, fn string) {
	// Filter and capitalize each segment
	var parts []string
	for seg := range strings.SplitSeq(relPath, "/") {
		for chunk := range strings.SplitSeq(seg, ".") {
			chunk = sanitizeIdentifier(chunk)
			if len(chunk) > 0 {
				parts = append(parts, capitalize(chunk))
			}
		}
	}

	id = strings.Join(parts, "")
	fn = strings.ReplaceAll(relPath, "/", "-")

	// Resolve collisions with numeric suffix
	newID := id
	newFN := fn
	for i := range 1000 {
		_, found := identifiers[newID]
		if !found {
			break
		}
		newID = id + strconv.Itoa(i)
	}
	for i := range 1000 {
		_, found := filenames[newFN]
		if !found {
			break
		}
		newFN = fn + strconv.Itoa(i)
	}

	return newID, newFN
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
	return relPath[:len(relPath)-len(ext)]
}

// createLinks creates symbolic links for assets in the output directory.
func createLinks(assets []asset, output, cacheDir string) error {
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
			// Create hard-link in assets directory
			target := filepath.Join(assetsDir, asset.Filename)
			// Remove existing file/link if present
			os.Remove(target)
			err := os.Link(asset.AbsPath, target)
			if err != nil {
				return fmt.Errorf("E087: Failed to create hard-link: %w", err)
			}
		} else {
			// Create hard-link in www directory
			target := filepath.Join(wwwDir, asset.RelPath)
			err := os.MkdirAll(filepath.Dir(target), 0o755)
			if err != nil {
				return fmt.Errorf("E087: Failed to create www subdirectory: %w", err)
			}
			// Remove existing file/link if present
			os.Remove(target)
			err = os.Link(asset.AbsPath, target)
			if err != nil {
				return fmt.Errorf("E087: Failed to create hard-link: %w", err)
			}
		}
	}

	return nil
}

type uint128 struct {
	Hi, Lo uint64
}

func uint128From16Bytes(b [16]byte) uint128 {
	return uint128{
		binary.LittleEndian.Uint64(b[8:]),
		binary.LittleEndian.Uint64(b[:8]),
	}
}

// computeImoHash computes the ImoHash for a file (128 bits)
// Uses github.com/kalafut/imohash.
func computeImoHashEtag(path string) (uint128, string, error) {
	sum, err := imohash.SumFile(path)
	if err != nil {
		return uint128{0, 0}, "", fmt.Errorf("imohash.SumFile: %w", err)
	}

	u128 := uint128From16Bytes(sum)
	etag := computeETag(sum)

	return u128, etag, nil
}

// computeETag generates an ETag from ImoHash
// Uses base91 encoding for compact representation.
func computeETag(hash [16]byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&()*+,./:;<=>?@[]^_ {|}~'"
	encoder := base91.NewEncoding(alphabet)
	b91 := encoder.EncodeToString(hash[:]) // the base91-encoded hash is 20 bytes
	return b91[:9]                         // truncate 9 bytes out of 20
}

func computeHashesETags(assets []asset) ([]asset, error) {
	// Compute hashes and ETags
	for i := range assets {
		hash, etag, err := computeImoHashEtag(assets[i].AbsPath)
		if err != nil {
			return nil, err
		}
		assets[i].ImoHash = hash
		assets[i].ETag = etag
	}
	return assets, nil
}
