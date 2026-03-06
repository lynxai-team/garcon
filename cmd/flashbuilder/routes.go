// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
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
		if len(assets[i].Route) == len(assets[j].Route) {
			return assets[i].Route < assets[j].Route // for deterministic result
		}
		return len(assets[i].Route) > len(assets[j].Route)
	})

	routes := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		routes[asset.Route] = struct{}{}
	}

	shortcuts := make([]asset, 0, len(assets))
	for a := range slices.Values(assets) {
		if len(a.MIME) >= 8 { // this trick is to avoid complex checking
			switch a.MIME[:8] { // in case of "charset=utf-8" presence
			case "text/css", "text/jav", "font/wof", "font/ttf":
				continue // no shortcut for CSS, JS and font files
			}
		}

		shortPath := generateShortcut(a.Route)
		_, found := routes[shortPath]
		if !found {
			a.IsShortcut = true
			a.Route = ""
			a.Route = shortPath
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
