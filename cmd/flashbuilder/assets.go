// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
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
	"os"
	"path"
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

const maxAssetSize = math.MaxInt32 // 2_147_483_647 Bytes = 2 GB

// asset represents a static asset with all pre-computed metadata.
type asset struct {
	Identifier string // Go identifier (e.g., "assetFaviconIco")
	Path       string // relative path to flags.Input used as Route
	VariantExt string // the variant Path has an extra extension ".br" ".avif" ".webp"
	MIME       string // Detected MIME type (e.g., "text/html"), used for Content-Type

	Form map[string]struct{} // contact-form API endpoint found in the HTML

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

// Define a structured type for items sent over the channel.
type fileItem struct {
	path string
	size int64
}

const workers = 4

// ErrSkip is a sentinel error to signal that a file should be skipped
// without logging it as a severe error (e.g., sockets, devices).
var ErrSkip = errors.New("skip")

func (a *asset) isVariant() bool {
	return a.VariantExt != ""
}

// processItem creates an asset from a file path.
// It handles the checks for special file types and returns ErrSkip for them.
// This logic is extracted from the worker to reduce clutter.
func processItem(input fs.FS, file fileItem, csp string) (*asset, error) {
	mimeType := detectMIME(input, file.path)
	isHTML, isIndex, c, form := extractHTML(input, file.path, mimeType)
	if isHTML && c == "" {
		c = csp
	}

	hash, etag, err := computeImoHashEtag(input, file.path)
	if err != nil {
		return nil, err
	}

	return &asset{
		Path:    file.path, // relative to input (also used as the request endpoint even if the variant is embedded)
		Size:    file.size,
		MIME:    mimeType,
		Hash:    hash,
		ETag:    etag,
		IsHTML:  isHTML,
		IsIndex: isIndex,
		CSP:     c,    // Content-Security-Policy (HTTP header)
		Form:    form, // Contact-form API endpoint
	}, nil
}

// discover walks the input directory and collects all files.
// Parallelized using errgroup. Accepts fs.FS for testability.
// Returns assets sorted by relative path for deterministic ordering.
func discover(input fs.FS, csp string) ([]asset, error) {
	var assets []asset
	var mu sync.Mutex

	// Initialize errgroup for concurrent processing
	g, ctx := errgroup.WithContext(context.Background())

	// Buffered channel to improve throughput.
	paths := make(chan fileItem, workers*2)

	// Worker pool to process paths (Consumer)
	for range workers {
		g.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case p, ok := <-paths:
					if !ok {
						return nil // Channel closed
					}

					// processItem handles the logic for a single file
					a, err := processItem(input, p, csp)
					if err != nil {
						// Use sentinel error to distinguish skips from failures
						if errors.Is(err, ErrSkip) {
							continue // Skip without logging
						}
						return err // Real error, stop the group
					}

					mu.Lock()
					assets = append(assets, *a)
					mu.Unlock()
				}
			}
		})
	}

	// Walk the filesystem (Producer)
	// This runs synchronously in the main goroutine.
	walkErr := fs.WalkDir(input, ".", func(assetPath string, entry fs.DirEntry, err error) error {
		if err != nil { // we cannot propagate error to errgroup context because
			return err // fs.WalkDir does not accept context => we check ctx.Done
		}

		if entry.IsDir() {
			return nil // skip directories
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		mode := info.Mode()
		if mode&os.ModeSocket != 0 || mode&os.ModeDevice != 0 || mode&os.ModeNamedPipe != 0 {
			return nil // skip special files (sockets, devices, pipes)
		}

		if info.Size() > maxAssetSize {
			slog.Info("skip asset", "size", info.Size(), "max", toHuman(maxAssetSize))
			return nil // security: no asset larger than 2 GB
		}

		select { // Send to worker
		case paths <- fileItem{path: assetPath, size: info.Size()}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	})

	// Close the channel to signal workers to stop.
	// This must happen after the producer (WalkDir) finishes.
	close(paths)

	// Wait for workers to finish.
	// g.Wait returns error if any worker returns error.
	waitErr := g.Wait()

	// Return walk or worker error
	if walkErr != nil {
		return nil, walkErr
	}
	if waitErr != nil {
		return nil, waitErr
	}

	// Sort by route for deterministic ordering
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Path < assets[j].Path
	})

	return assets, nil
}

// extractFromFS wraps [extractFromHTML].
func extractFromFS(input fs.FS, assetPath string) (csp string, actions map[string]struct{}) {
	// Re-open to read full content (or reset reader)
	f, err := input.Open(assetPath)
	if err != nil {
		slog.Warn("extractFromFS input.Open", "path", assetPath, "err", err)
		return "", nil
	}
	defer f.Close()
	return extractFromHTML(f)
}

// extractFromHTML parses HTML and returns first CSP and unique <form> actions.
func extractFromHTML(r io.Reader) (csp string, actions map[string]struct{}) {
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if errors.Is(err, io.EOF) {
				slog.Warn("HTML parse error", "error", err)
			}
			return csp, actions // EOF reached
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
					csp = html.UnescapeString(content)
				}
			case "form":
				// Collect unique action attributes.
				for _, a := range t.Attr {
					if strings.EqualFold(a.Key, "action") {
						act := html.UnescapeString(a.Val)
						if _, ok := actions[act]; !ok {
							if actions == nil {
								actions = map[string]struct{}{act: {}}
								break
							}
							actions[act] = struct{}{}
						}
						break
					}
				}
			}
		}
	}
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

func extractHTML(input fs.FS, assetPath, mimeType string) (isHTML, isIndex bool, csp string, form map[string]struct{}) {
	isHTML = strings.HasSuffix(mimeType, "/html") || // text/html
		strings.HasSuffix(mimeType, "html+xml") // application/xhtml+xml

	isIndex = isHTML && strings.HasSuffix(assetPath, "index.html")

	if isHTML {
		csp, form = extractFromFS(input, assetPath)
	}

	return isHTML, isIndex, csp, form
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

// setIdentifiers sets identifiers.
func setIdentifiers(assets []asset) {
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

// estimateFrequencyScore estimates request frequency for switch case ordering
// Higher frequency assets appear first in switch statements for better branch prediction.
func estimateFrequencyScore(assetPath string, isEmbed bool) int {
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
	if isEmbed {
		score += 200
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

func uint128From16Bytes(b [16]byte) uint128 {
	return uint128{
		binary.LittleEndian.Uint64(b[8:]),
		binary.LittleEndian.Uint64(b[:8]),
	}
}

// computeImoHash computes the ImoHash for a file (128 bits).
func computeImoHashEtag(input fs.FS, assetPath string) (uint128, string, error) {
	f, err := input.Open(assetPath)
	if err != nil {
		return uint128{0, 0}, "", fmt.Errorf("computeImoHashEtag input.Open: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return uint128{0, 0}, "", fmt.Errorf("computeImoHashEtag f.Stat: %w", err)
	}

	ff, ok := f.(io.ReaderAt)
	if !ok {
		return uint128{0, 0}, "", errors.New("computeImoHashEtag: cannot access fs.File as io.ReaderAt")
	}

	sr := io.NewSectionReader(ff, 0, fi.Size())
	sum, err := imohash.SumSectionReader(sr)
	if err != nil {
		return uint128{0, 0}, "", fmt.Errorf("imohash.SumSectionReader: %w", err)
	}

	u128 := uint128From16Bytes(sum)
	etag := computeETag(sum)

	return u128, etag, nil
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

// computeImoHashEtag sets the ImoHash and ETag for each asset.
// Not very IO intensive because ImoHash reads three file chunks.
// However, lets's speed up this step this step with parallel workers.
// The slice assets[i] can be safely concurrency accessed
// because it does not move during this processing.
func computeHashesETags(input fs.FS, assets []asset) ([]asset, error) {
	var g errgroup.Group

	nA := len(assets)      // nA = number of assets
	nW := min(nA, workers) // nW = number of workers (no more than assets)

	if nW <= 0 {
		return assets, nil // nothing to do
	}

	next := 0
	for w := range nW {
		g.Go(func() error {
			first := next
			next = ((w + 1) * nA) / nW // next == nA for the last worker (w == nW-1)

			for i := first; i < next; i++ {
				hash, etag, err := computeImoHashEtag(input, assets[i].Path)
				if err != nil {
					return err
				}
				assets[i].Hash = hash
				assets[i].ETag = etag
			}
			return nil
		})
	}

	err := g.Wait()
	return assets, err
}
