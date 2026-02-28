// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"slices"
	"sort"
	"strconv"
)

// handlers is sed to render the handlers and get array.
type handlers struct {
	PrevEntry string
	Entry     string
	Routes    []asset
	Length    int
}

// buildRoutesByLength groups routes by length, sorted by frequency score.
func buildRoutesByLength(assets []asset, size int) []map[string][]asset {
	routesByLen := make([]map[string][]asset, size)

	// 1. group routes by length
	for _, a := range assets {
		routeLen := len(a.RelPath) // relative path without leading slash

		method := "GET"
		if a.Form != "" {
			method = "POST"
		}

		if routesByLen[routeLen] == nil {
			routesByLen[routeLen] = map[string][]asset{method: []asset{a}}
		} else {
			routesByLen[routeLen][method] = append(routesByLen[routeLen][method], a)
		}
	}

	// 2. sort by frequency score within each length group
	for routeLen, routes := range routesByLen {
		for method, assets := range routes {
			sort.Slice(assets, func(i, j int) bool {
				return assets[i].Frequency > assets[j].Frequency
			})
			routesByLen[routeLen][method] = assets
		}
	}

	return routesByLen
}

// buildGet generates get arrays for HTTP and HTTPS
// We use `index = assetRouteLen + 1` to eliminate the runtime slash removal.
func buildGet(assets []asset, maxLen int) []handlers {
	routesByLen := buildRoutesByLength(assets, maxLen+1)
	get := make([]handlers, maxLen+2)
	getEntry := "notFound"

	for i := range get {
		// index (of the get array) is the length of the request path (including leading slash)
		// the asset routes are relative paths (no leading slash)
		routeLen := max(0, i-1) // subtract the leading slash
		assets := routesByLen[routeLen]["GET"]

		prevEntry := getEntry
		if len(assets) > 0 {
			if routeLen == 0 {
				getEntry = "serveIndexHtml"
			} else {
				getEntry = "getLen" + strconv.Itoa(routeLen)
			}
		}

		get[i] = handlers{
			Length:    routeLen,
			Entry:     getEntry,
			PrevEntry: prevEntry,
			Routes:    assets,
		}
	}

	return get
}

func addShortcutPaths(assets []asset) []asset {
	paths := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		paths[asset.RelPath] = struct{}{}
	}

	shortcuts := make([]asset, 0, len(assets))
	for a := range slices.Values(assets) {
		shortPath := generateShortcut(a.RelPath)
		_, found := paths[shortPath]
		if !found {
			a.IsShortcut = true
			a.AbsPath = ""
			a.RelPath = shortPath
			shortcuts = append(shortcuts, a)
			paths[shortPath] = struct{}{}
		}
	}
	return append(assets, shortcuts...)
}

// computeMaxLen calculates the maximum path length for get array sizing.
func computeMaxLen(assets []asset) int {
	maxLen := 0
	for _, asset := range assets {
		if len(asset.RelPath) > maxLen {
			maxLen = len(asset.RelPath)
		}
	}
	return maxLen
}
