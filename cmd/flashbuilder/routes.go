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
		assets[i].Route = escapePathSegmentsPerf(assets[i].Path)
	}
}

// escapePathSegments splits the path into
// segments, escapes each segment individually, and joins them back.
func escapePathSegments(path string) string {
	segments := strings.Split(path, "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	return strings.Join(segments, "/")
}

// escapePathSegmentsPerf escapes individual segments of a provided route,
// preserving the path separators '/'. It uses a zero-allocation fast path
// for clean paths and a strings.Builder for dirty paths.
// This function is high-performant and adheres to RFC 3986 for path segments.
func escapePathSegmentsPerf(path string) string {
	// Fast path: check if any escaping is needed.
	// We scan for any character that is NOT a separator '/' and NOT an unreserved character.
	// Unreserved characters (RFC 3986): a-z, A-Z, 0-9, '-', '.', '_', '~'.
	for i := range len(path) {
		c := path[i]
		if c == '/' {
			continue // Separator, keep it.
		}
		// Check safe characters (unreserved set).
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			continue
		}
		// Found a character that needs escaping.
		// Fall through to the slow path (building).
		goto build
	}
	// Path is already safe (or empty), return the original string (zero allocation).
	return path

build:
	// Slow path: build a new string with escaping.
	var b strings.Builder
	// Estimate capacity (worst case: every char needs %XX -> 3x length).
	b.Grow(len(path) + len(path)) // Allocate a bit more to reduce chance of growing.

	// Hex table for percent encoding.
	const hex = "0123456789ABCDEF"

	for i := range len(path) {
		c := path[i]
		if c == '/' {
			b.WriteByte('/')
			continue
		}
		// Check safe
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
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
