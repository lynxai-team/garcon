// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/kalafut/imohash"
	"github.com/mtraver/base91"
	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"
)

const maxAssetSize = math.MaxInt32 // 2_147_483_647 Bytes = 2 GB

// asset represents a static asset with all pre-computed metadata.
type asset struct {
	Identifier string // Go identifier (e.g., "assetFaviconIco")
	Path       string // relative path to flags.Input used as Route
	VariantExt string // the variant Path has an extra extension ".br" ".avif" ".webp"
	MIME       string // Detected MIME type (e.g., "text/html"), used for Content-Type

	API map[string]struct{} // API endpoints (contact-form found in HTML) for the POST routes

	// headers
	CSP       string  // Content-Security-Policy header value
	ETag      string  // Base91 ETag for conditional GET (quoted)
	Hash      uint128 // Content hash from imohash
	Size      int64   // File size in bytes
	Frequency int     // Request frequency score for switch ordering

	IsEmbedEligible bool // Selected for embedding within budget
	IsDuplicate     bool // Content matches another asset
	IsShortcut      bool

	// HTML
	IsHTML  bool // Is HTML content (for CSP injection)
	IsIndex bool // Is index file (e.g., index.html)
}

// discoverAssets walks the input directory and collects all files.
// It uses errgroup.SetLimit to bound concurrency,
// preventing excessive goroutine spawning and memory exhaustion,
// by blocking the WalkDir producer when the `workers` limit is reached.
func discoverAssets(input fs.FS, csp string) ([]asset, error) {
	var assets []asset
	var mu sync.Mutex

	// Initialize errgroup with context for cancellation.
	g, ctx := errgroup.WithContext(context.Background())

	// Set the concurrency limit: ensure we do not spawn more than 'workers' goroutines.
	workers := max(2, runtime.NumCPU()/2) // NumCPU = number of logical CPUs
	g.SetLimit(workers)

	// Walk the filesystem
	err := fs.WalkDir(input, ".", func(assetPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr // Propagate I/O errors immediately.
		}

		// If the context is canceled by another worker => we stop immediately.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if entry.IsDir() {
			return nil // skip directories
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		mode := info.Mode()
		const skipMask = os.ModeSocket | os.ModeDevice | os.ModeNamedPipe
		if mode&skipMask != 0 {
			slog.Info("skip special files (sockets, devices, pipes)", "mode", mode)
			return nil
		}

		if info.Size() > maxAssetSize {
			slog.Info("skip asset", "size", info.Size(), "max", toHuman(maxAssetSize))
			return nil // security: no asset larger than 2 GB
		}

		// Spawn a processing task.
		// g.Go will block if the limit is reached, throttling the WalkDir.
		// If the context is canceled, g.Go returns ctx.Err() immediately.
		g.Go(func() error {
			a, err := newAsset(input, assetPath, csp)
			if err != nil {
				return err
			}

			a.Size = info.Size()

			mu.Lock()
			assets = append(assets, *a) // Append to slice with mutex protection
			mu.Unlock()
			return nil
		})

		return nil
	})
	// If WalkDir returned an error (e.g., permission denied), we return it.
	if err != nil {
		return nil, err
	}

	// Wait for all outstanding goroutines to finish.
	// g.Wait returns the first error (if any), canceling the context.
	err = g.Wait()
	if err != nil {
		return nil, err
	}

	return assets, nil
}

// newAsset creates an asset from a file path.
func newAsset(input fs.FS, assetPath, csp string) (*asset, error) {
	mimeType := detectMIME(input, assetPath)

	isHTML, isIndex, c, endpoints := extractHTML(input, assetPath, mimeType)
	if isHTML && c == "" {
		c = csp
	}

	hash, etag, err := computeImoHashEtag(input, assetPath)
	if err != nil {
		return nil, err
	}

	return &asset{
		Path:      assetPath, // relative to input (also used as the request endpoint even if the variant is embedded)
		MIME:      mimeType,
		Hash:      hash,
		ETag:      etag,
		IsHTML:    isHTML,
		IsIndex:   isIndex,
		CSP:       c,         // Content-Security-Policy (HTTP header)
		API:       endpoints, // Contact-form API endpoints (POST requests)
		Frequency: estimateFrequencyScore(assetPath),
	}, nil
}

// estimateFrequencyScore estimates request frequency for switch case ordering
// Higher frequency assets appear first in switch statements for better branch prediction.
func estimateFrequencyScore(assetPath string) int {
	score := 0

	if assetPath == "" || assetPath == "index.html" {
		score += 1000
	}
	if strings.Contains(assetPath, "favicon.") {
		score += 900
	}
	if strings.HasSuffix(assetPath, ".css") {
		score += 800
	}
	if strings.HasSuffix(assetPath, ".js") {
		score += 600
	}
	if strings.Contains(assetPath, "index.html") {
		score += 500
	}
	if strings.Contains(assetPath, "logo.") {
		score += 400
	}

	// Path complexity penalty
	score -= 5 * len(assetPath)
	score -= 30 * strings.Count(assetPath, "/")

	// Low-traffic extensions penalty
	lowTraffic := []string{".map", ".zip", ".pdf", ".doc", ".xls", ".tar"}
	for _, ext := range lowTraffic {
		if strings.HasSuffix(assetPath, ext) {
			score -= 100
			break
		}
	}

	return score
}

func extractHTML(input fs.FS, assetPath, mimeType string) (isHTML, isIndex bool, csp string, endpoints map[string]struct{}) {
	isHTML = strings.HasSuffix(mimeType, "/html") || // text/html
		strings.HasSuffix(mimeType, "html+xml") // application/xhtml+xml

	isIndex = isHTML && strings.HasSuffix(assetPath, "index.html")

	if isHTML {
		csp, endpoints = parseHTML(input, assetPath)
	}

	return isHTML, isIndex, csp, endpoints
}

// parseHTML parses HTML and returns first CSP and unique <form> actions.
func parseHTML(input fs.FS, assetPath string) (csp string, endpoints map[string]struct{}) {
	// Re-open to read full content (or reset reader)
	f, err := input.Open(assetPath)
	if err != nil {
		slog.Warn("extractFromFS input.Open", "path", assetPath, "err", err)
		return "", nil
	}
	defer f.Close()

	z := html.NewTokenizer(f)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if errors.Is(err, io.EOF) {
				slog.Warn("HTML parse error", "error", err)
			}
			return csp, endpoints // EOF reached
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			switch strings.ToLower(t.Data) {
			case "meta":
				if csp != "" {
					continue // already extracted
				}
				// Look for CSP meta tag.
				var httpEquiv, content string
				for _, a := range t.Attr {
					switch strings.ToLower(a.Key) {
					case "http-equiv":
						httpEquiv = a.Val
					case "content":
						content = a.Val
					}
				}
				if strings.EqualFold(httpEquiv, "Content-Security-Policy") && content != "" {
					csp = validCSP(html.UnescapeString(content))
				}
			case "form":
				// Collect unique action attributes.
				for _, a := range t.Attr {
					if strings.EqualFold(a.Key, "action") {
						api, err := validEndpoint(assetPath, html.UnescapeString(a.Val))
						if err != nil {
							slog.Info("skip <form>", "err", err)
							break
						}
						if _, ok := endpoints[api]; !ok {
							if endpoints == nil {
								endpoints = map[string]struct{}{api: {}}
								break
							}
							endpoints[api] = struct{}{}
						}
						break
					}
				}
			}
		}
	}
}

func validCSP(csp string) string {
	for i := range len(csp) {
		// Valid CSP only contains visible ASCII characters (Space 0x20 to Tilde 0x7E)
		if csp[i] < 0x20 || csp[i] > 0x7E {
			slog.Info("skip invalid", "CSP", csp)
			return ""
		}
	}
	return csp
}

// validEndpoint sanitizes the form action endpoint.
func validEndpoint(assetPath, endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", endpoint, err)
	}

	sanitized := path.Clean(u.Path)
	if !path.IsAbs(sanitized) {
		sanitized = path.Join(path.Dir(assetPath), sanitized)
	}

	if sanitized == "" {
		return "", fmt.Errorf("empty after sanitization %q", endpoint)
	}

	if sanitized[0] == '/' {
		sanitized = sanitized[1:] // drop leading slash
	}

	if sanitized != "" && !fs.ValidPath(sanitized) {
		return "", fmt.Errorf("invalid path %q", sanitized)
	}

	return sanitized, nil
}

// detectMIME determines the MIME type for a file
// Step 1: Extension-based lookup
// Step 2: Content sniffing
// Step 3: Fallback to application/octet-stream.
func detectMIME(input fs.FS, assetPath string) string {
	// search by extension
	ext := path.Ext(assetPath)
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType
		}
	}

	// sniff first 512 bytes
	file, err := input.Open(assetPath)
	if err != nil {
		slog.Warn("detectMIME input.Open", "path", assetPath, "error", err)
		return ""
	}
	defer file.Close()

	// Safe read using bufio or io.LimitReader
	// Limit to 512 bytes for MIME sniffing
	sniffReader := io.LimitReader(file, 512)
	sniffBuf, err := io.ReadAll(sniffReader)
	if err != nil {
		slog.Warn("detectMIME io.ReadAll", "path", assetPath, "error", err)
		return ""
	}

	// Detect MIME type
	mimeType := http.DetectContentType(sniffBuf)
	if mimeType != "" {
		return mimeType
	}

	// Step 3: Fallback
	return "application/octet-stream"
}

// generateIdentifiers sets identifiers.
func generateIdentifiers(assets []asset) {
	identifiers := make(existing, len(assets))
	for i := range assets {
		assets[i].Identifier = identifiers.generateIdentifier(assets[i].Path)
	}
}

type existing map[string]struct{}

// generateIdentifier creates a valid Go identifier from a relative path
// Deterministic sanitization ensures reproducibility.
func (e existing) generateIdentifier(inPath string) string {
	var result strings.Builder
	capitalize := true
	for _, r := range inPath {
		if unicode.IsLetter(r) {
			if capitalize {
				r = unicode.ToUpper(r)
				capitalize = false
			}
			result.WriteRune(r)
		} else if unicode.IsDigit(r) {
			result.WriteRune(r)
		} else {
			capitalize = true
		}
	}
	id := result.String()
	id = e.resolveCollision(id)
	return id
}

// resolveCollision resolves collisions with numeric suffix.
func (e existing) resolveCollision(original string) string {
	value := original
	const N = 100_000
	for i := range N {
		_, found := e[value]
		if !found {
			e[value] = struct{}{}
			return value
		}
		value = original + strconv.Itoa(i)
	}
	slog.Error("Collision could not be resolved", "value", value, "limit", N)
	return value
}

// deduplicate identifies duplicate assets based on content hash.
func deduplicate(assets []asset) []asset {
	hashMap := make(map[uint128][]int) // Map hash -> slice of asset indices

	// Sort by route length
	sort.Slice(assets, func(i, j int) bool {
		return len(assets[i].Path) < len(assets[j].Path)
	})

	// Group assets by hash
	for i, a := range assets {
		key := a.Hash
		hashMap[key] = append(hashMap[key], i)
	}

	// Mark duplicates
	for _, indices := range hashMap {
		if len(indices) > 1 {
			canonicalIdx := indices[0]
			canonical := assets[canonicalIdx]
			// TODO: For now, assume hash is enough, but we should check content
			for i := 1; i < len(indices); i++ {
				idx := indices[i]
				assets[idx].IsDuplicate = true
				assets[idx].Identifier = canonical.Identifier
			}
		}
	}
	return assets
}

// generateShortcut creates an extensionless shortcut and
// clean URLs like "/about" instead of "/about/index.html".
func generateShortcut(inPath string) string {
	// Root index has no shortcut
	if inPath == "index.html" {
		return ""
	}

	// Index files in subdirectories
	if before, ok := strings.CutSuffix(inPath, "/index.html"); ok {
		return before
	}

	// Extensionless shortcuts
	ext := path.Ext(inPath)
	return inPath[:len(inPath)-len(ext)]
}

type uint128 struct {
	Hi, Lo uint64
}

// String stringifies uint128:
// - returns 22‑character Base64‑URL filename‑safe.
// - Zero‑allocation: only a fixed [22]byte lives on the stack.
func (u uint128) String() string {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[:8], u.Hi)
	binary.BigEndian.PutUint64(buf[8:], u.Lo)
	enc := base64.URLEncoding.WithPadding(base64.NoPadding)
	return enc.EncodeToString(buf[:])
}

func uint128From16Bytes(b [16]byte) uint128 {
	return uint128{
		binary.LittleEndian.Uint64(b[8:]),
		binary.LittleEndian.Uint64(b[:8]),
	}
}

// computeImoHash computes the ImoHash for a file (128 bits).
func computeImoHashEtag(input fs.FS, assetPath string) (hash uint128, etag string, err error) {
	f, err := input.Open(assetPath)
	if err != nil {
		return uint128{0, 0}, "", fmt.Errorf("computeImoHashEtag input.Open: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return uint128{0, 0}, "", fmt.Errorf("computeImoHashEtag f.Stat: %w", err)
	}

	// use ReaderAt for efficiency, fallback to buffered reading if not supported (fstest).
	readerAt, ok := f.(io.ReaderAt)
	if !ok { // fallback: read content into memory with a limit to prevent OOM.
		const maxMemoryLoad = 100 * 1024 * 1024 // 100 MB
		limitedReader := io.LimitReader(f, maxMemoryLoad)
		buf, er := io.ReadAll(limitedReader)
		if er != nil {
			return uint128{0, 0}, "", fmt.Errorf("computeImoHashEtag io.ReadAll: %w", er)
		}
		readerAt = bytes.NewReader(buf)
	}

	sr := io.NewSectionReader(readerAt, 0, info.Size())
	sum, err := imohash.SumSectionReader(sr)
	if err != nil {
		return uint128{0, 0}, "", fmt.Errorf("imohash.SumSectionReader: %w", err)
	}

	hash = uint128From16Bytes(sum)
	etag = computeETag(sum)

	return hash, etag, nil
}

// base91Alphabet contains 91 ASCII characters that are safe for POSIX filenames (Linux and macOS).
// POSIX filename may contain any byte except NUL (\0) and the forward slash (/).
// The printable ASCII range (0x20 – 0x7E) gives 95 characters.
// Removing the only forbidden character (/) leaves 94.
// To reach exactly 91 distinct characters we drop three more printable ASCII characters that are rarely needed in an encoding:
//   - a space ( )
//   - a double‑quote (")
//   - a back‑slash (\)
//
// The remaining 91 characters are all pure ASCII (code points 0x21–0x7E, excluding the four omitted ones) and are therefore safe for any POSIX‑compliant filesystem.
// Windows (NTFS, FAT, exFAT) is not supported because it forbids the characters < > : " / \ | ? * and the NUL byte (Windows: only 85 ASCII characters).
const base91Alphabet = "!#$%&'()*+,-.0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[]^_`abcdefghijklmnopqrstuvwxyz{|}~"

func computeETag(hash [16]byte) string {
	encoder := base91.NewEncoding(base91Alphabet)
	b91 := encoder.EncodeToString(hash[:])
	return b91
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
