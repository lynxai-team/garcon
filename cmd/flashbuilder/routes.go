// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// handlers is used to render the handlers and get array.
type handlers struct {
	PrevEntry string
	Entry     string
	Routes    []asset
	Length    int
}

func computeMaxLenGet(assets []asset) int {
	maxLenG := 0
	for _, asset := range assets {
		maxLenG = max(maxLenG, len(asset.Route))
	}
	return maxLenG
}

func computeMaxLenPost(assets []asset) int {
	maxLenP := 0
	for _, a := range assets {
		for route := range a.API {
			maxLenP = max(maxLenP, len(route))
		}
	}
	return maxLenP
}

func buildGetPostDispatch(assets []asset) (get, post []handlers, _ error) {
	maxLenG := computeMaxLenGet(assets)
	maxLenP := computeMaxLenPost(assets)

	const maxLenSecurity = 10_000
	if maxLenG > maxLenSecurity {
		return nil, nil, fmt.Errorf("the largest asset path is %d > %d", maxLenG, maxLenSecurity)
	}
	if maxLenP > maxLenSecurity {
		return nil, nil, fmt.Errorf("the largest endpoint route is %d > %d", maxLenP, maxLenSecurity)
	}

	// Generate get and post arrays
	get = buildGet(assets, maxLenG)
	post = buildPost(assets, maxLenP)

	return get, post, nil
}

// buildGet generates the dispatch array for the GET handlers.
func buildGet(assets []asset, maxLen int) []handlers {
	routesByLen := buildGetRoutesByLength(assets, maxLen+1)
	get := make([]handlers, maxLen+2)
	function := "notFound"

	for i := range get {
		// index (of the get array) is the length of the request path (including leading slash)
		// the asset routes are relative paths (no leading slash)
		routeLen := max(0, i-1) // subtract the leading slash
		if routeLen >= len(routesByLen) {
			// Handle out of bounds
			continue
		}
		assets := routesByLen[routeLen]

		prevEntry := function
		if len(assets) > 0 {
			if routeLen == 0 {
				function = "getIndexHtml"
			} else {
				function = "getLen" + strconv.Itoa(routeLen)
			}
		}

		get[i] = handlers{
			Length:    routeLen,
			Entry:     function,
			PrevEntry: prevEntry,
			Routes:    assets,
		}
	}
	return get
}

// buildPost generates the dispatch array for the POST handlers.
func buildPost(assets []asset, maxLen int) []handlers {
	routesByLen := buildPostRoutesByLength(assets, maxLen+1)
	post := make([]handlers, maxLen+2)
	atLeastOnePost := false

	for i := range post {
		routeLen := max(0, i-1)
		if routeLen >= len(routesByLen) {
			continue
		}
		assets := routesByLen[routeLen]

		function := "notFound"
		if len(assets) > 0 {
			if routeLen == 0 {
				function = "notFound"
			} else {
				function = "postLen" + strconv.Itoa(routeLen)
				atLeastOnePost = true
			}
		}

		post[i] = handlers{
			Length:    routeLen,
			Entry:     function,
			PrevEntry: "notFound",
			Routes:    assets,
		}
	}

	if atLeastOnePost {
		return post
	}
	return nil
}

// buildGetRoutesByLength groups routes by length, sorted by frequency score.
func buildGetRoutesByLength(assets []asset, size int) [][]asset {
	routesByLen := make([][]asset, size+1) // allocate size+1 to avoid panic

	// group routes by length
	for _, a := range assets {
		routeLen := len(a.Route)
		routesByLen[routeLen] = append(routesByLen[routeLen], a)
	}

	// sort by frequency score within each length group
	for _, assets := range routesByLen {
		if len(assets) == 0 {
			continue
		}
		sort.Slice(assets, func(i, j int) bool {
			return assets[i].Frequency > assets[j].Frequency
		})
	}

	return routesByLen
}

// buildPostRoutesByLength groups routes by length for POST requests.
func buildPostRoutesByLength(assets []asset, size int) [][]asset {
	routesByLen := make([][]asset, size+1)
	exist := existing{}

	for _, a := range assets {
		for route := range a.API {
			routeLen := len(route)

			// skip duplicated routes
			if slices.ContainsFunc(routesByLen[routeLen], func(a asset) bool { return route == a.Route }) {
				continue
			}

			modifiedAsset := a
			modifiedAsset.Identifier = exist.generateIdentifier(route)
			modifiedAsset.Route = route
			routesByLen[routeLen] = append(routesByLen[routeLen], modifiedAsset)
		}
	}
	// no sorting logic for POST endpoints yet, leaving as is.
	return routesByLen
}

func addShortcutRoutes(assets []asset) []asset {
	// Sort by route length (from larger to smaller)
	// to favor larger route in case of conflict.
	// Example: "about" will be the shortcut of "about.html" rather than "about.md"
	sort.Slice(assets, func(i, j int) bool {
		if len(assets[i].Path) == len(assets[j].Path) {
			return assets[i].Path < assets[j].Path // for deterministic result
		}
		return len(assets[i].Path) > len(assets[j].Path)
	})

	routes := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		routes[asset.Path] = struct{}{}
	}

	shortcuts := make([]asset, 0, len(assets))
	for a := range slices.Values(assets) {
		if len(a.MIME) >= 8 { // this trick is to avoid complex checking
			switch a.MIME[:8] { // in case of "charset=utf-8" presence
			case "text/css", "text/jav", "font/wof", "font/ttf":
				continue // no shortcut for CSS, JS and font files
			}
		}

		shortPath := generateShortcut(a.Path)
		_, found := routes[shortPath]
		if !found {
			a.IsShortcut = true
			a.Path = ""
			a.Path = shortPath
			shortcuts = append(shortcuts, a)
			routes[shortPath] = struct{}{}
		}
	}
	return append(assets, shortcuts...)
}

// generateShortcut creates an extensionless shortcut and
// clean URLs like:
// - "about/index.html" -> "about"
// - "about/index.md"   -> "about/index"
// - "about/offer.html" -> "about/offer"
// - "about/image.jpeg" -> "about/image"
// Note that "/about/index" is never the shortcut of "/about/index.html".
// The "/about/index" shortcut can be from another file like "/about/index.md".
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

func escapeRoutes(assets []asset) {
	for i := range assets {
		assets[i].Route = escapePathSegmentsPerf1(assets[i].Path)
	}
}

// escapePathSegmentsSimple splits the path into
// segments, escapes each segment individually, and joins them back.
func escapePathSegmentsSimple(path string) string {
	segments := strings.Split(path, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return strings.Join(segments, "/")
}

// escapePathSegmentsPerf1 is the high-performant variation of [escapePathSegmentsSimple].
// It escapes individual segments of a provided route, preserving the path separators '/'.
// It uses a zero-allocation fast path for clean paths and a strings.Builder for dirty paths.
// This function is high-performant and adheres to RFC 3986 for path segments.
func escapePathSegmentsPerf1(path string) string {
	// Fast path: check if any escaping is needed.
	// We scan for any character that is NOT a separator '/' and NOT an unreserved character.
	// Unreserved characters (RFC 3986): a-z, A-Z, 0-9, '-', '.', '_', '~'.
	for i := range len(path) {
		c := path[i]
		if c == '/' {
			continue // Separator, keep it.
		}
		// Check safe characters (unreserved set).
		if isUnreservedMask4(c) {
			continue
		}
		// Found a character that needs escaping.
		// Fall through to the slow path (building).
		goto slowPath
	}
	// Path is already safe (or empty), return the original string (zero allocation).
	return path

slowPath:
	// Slow path: build a new string with escaping.
	var b strings.Builder
	// Allocate more to reduce chance of growing (worst case: every char needs %XX -> 3x length).
	b.Grow(len(path) * 2)

	// Hex table for percent encoding.
	const hex = "0123456789ABCDEF"

	for i := range len(path) {
		c := path[i]
		if c == '/' {
			b.WriteByte('/')
			continue
		}
		// Check safe
		if isUnreservedMask4(c) {
			b.WriteByte(c)
			continue
		}
		// Escape unsafe character.
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xF])
	}
	return b.String()
}

// escapePathSegments is the high-performant variation of [escapePathSegmentsSimple].
// It escapes individual segments of a provided route, preserving the path separators '/'.
// It uses a zero-allocation fast path for clean paths and a strings.Builder for dirty paths.
// This function is high-performant and adheres to RFC 3986 for path segments.
func escapePathSegmentsPerf2(path string) string {
	// Hex table for percent encoding.
	const hex = "0123456789ABCDEF"
	// Buffer for slow path
	var b strings.Builder

	// Fast path: check if any escaping is needed.
	// We scan for any character that is NOT a separator '/' and NOT an unreserved character.
	// Unreserved characters (RFC 3986): a-z, A-Z, 0-9, '-', '.', '_', '~'.
	var i int
	for i = range len(path) {
		c := path[i]
		if c == '/' {
			continue // Separator, keep it.
		}
		// Check safe characters (unreserved set).
		if isUnreservedMask4(c) {
			continue
		}
		// Found a character that needs escaping.
		// Fall through to the slow path (building).
		// Allocate more to reduce chance of growing (worst case: every char needs %XX -> 3x length).
		b.Grow(len(path) * 2)
		b.WriteString(path[:i])
		// Escape unsafe character.
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xF])
		i++
		goto slowPath
	}
	// Path is already safe (or empty), return the original string (zero allocation).
	return path

slowPath: // build a new string with escaping.

	for ; i < len(path); i++ {
		c := path[i]
		if c == '/' {
			b.WriteByte('/')
			continue
		}
		// Check safe
		if isUnreservedMask4(c) {
			b.WriteByte(c)
			continue
		}
		// Escape unsafe character.
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0xF])
	}
	return b.String()
}

func isUnreservedSimple(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~'
}

// isUnreserved reports whether c is an “unreserved” character according to RFC-3986.
// The function works for any byte value; values ≥ 128 are automatically rejected.
func isUnreserved(c byte) bool {
	// 0-9
	if c >= '0' && c <= '9' {
		return true
	}

	// A-Z and a-z share the same range once bit 0x20 (the 6-th bit) is forced to 1.
	//   'A' = 0x41 -> 0x61  (a)
	//   'Z' = 0x5A -> 0x7A  (z)
	//   'a' = 0x61 -> 0x61  (a)
	//   'z' = 0x7A -> 0x7A  (z)
	if lc := c | 0x20; lc >= 'a' && lc <= 'z' {
		return true
	}

	// '-' (0x2D) and '.' (0x2E) are consecutive.
	if c >= '-' && c <= '.' {
		return true
	}

	// The only remaining ASCII symbols that are allowed.
	return c == '_' || c == '~'
}

// mask0 holds the bits for ASCII values 0-63.
// Bits that are set: 0x2D ('-'), 0x2E ('.'), 0x30-0x39 ('0'-'9').
const mask0 uint64 = 0x03FF600000000000

// mask1 holds the bits for ASCII values 64-127.
// Bits that are set: 0x41-0x5A ('A'-'Z'), 0x5F ('_'), 0x61-0x7A ('a'-'z'), 0x7E ('~').
const mask1 uint64 = 0x47FFFFFE87FFFFFE

// unreservedMask is a two-element array where the first element contains the
// bitmap for values 0-63 and the second element for 64-127.
var unreservedMask = [2]uint64{mask0, mask1}

// isUnreservedMask is a branch-free test that works for any byte value.
// Values ≥128 are automatically rejected because they fall outside the 128-bit bitmap.
func isUnreservedMask(c byte) bool {
	if c >= 128 { // outside the pre-computed range
		return false
	}
	// c>>6 selects the half (0 for 0-63, 1 for 64-127),
	// c&0x3F gives the bit position inside that half.
	return ((unreservedMask[c>>6] >> (c & 0x3F)) & 1) != 0
}

// 256-bit bitmap of RFC-3986 “unreserved” characters.
// Entry 0 -> bytes 0-63, entry 1 -> bytes 64-127, entries 2-3 (128-255) are zero.
var unreservedMask4 = [4]uint64{
	mask0, // '-' '.' '0'-'9' (bits 45,46,48-57)
	mask1, // 'A'-'Z' '_' 'a'-'z' '~' (bits 1-26,31,33-58,62)
	0, 0,  // 128-255: none are unreserved
}

// isUnreservedMask reports whether c is an RFC-3986 unreserved byte.
// The test is completely branch-free: the 256-bit bitmap is indexed by the
// high 2 bits (c>>6) and the result is a simple shift-and-test.
func isUnreservedMask4(c byte) bool {
	return ((unreservedMask4[c>>6] >> (c & 0x3F)) & 1) != 0
}
