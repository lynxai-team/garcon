// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"slices"
	"sort"
	"strconv"
)

// handlers is sed to render the handlers and dispatch array.
type handlers struct {
	PrevEntry string
	Entry     string
	Routes    []asset
	Length    int
}

// buildRoutesByLength groups routes by length, sorted by frequency score.
func buildRoutesByLength(assets []asset, size int) [][]asset {
	routesByLen := make([][]asset, size)

	// 1. group routes by length
	for _, asset := range assets {
		route := asset
		routeLen := len(route.RelPath) // relative path without leading slash
		routesByLen[routeLen] = append(routesByLen[routeLen], route)
	}

	// 2. sort by frequency score within each length group
	for i, routes := range routesByLen {
		sort.Slice(routes, func(i, j int) bool {
			return routes[i].Frequency > routes[j].Frequency
		})
		routesByLen[i] = routes
	}

	return routesByLen
}

// buildDispatch generates dispatch arrays for HTTP and HTTPS
// Dispatch index = route length + 1 (eliminates runtime slash removal).
func buildDispatch(assets []asset, maxLen int) []handlers {
	assetRoutesByLen := buildRoutesByLength(assets, maxLen+1)
	dispatch := make([]handlers, maxLen+2)
	dispatchEntry := "notFound"

	for i := range dispatch {
		// dispatch index is the length of the request path (including leading slash)
		// the asset routes are relative paths (no leading slash)
		assetRouteLen := max(0, i-1) // subtract the leading slash
		assetRoutes := assetRoutesByLen[assetRouteLen]

		prevEntry := dispatchEntry
		if len(assetRoutes) > 0 {
			if assetRouteLen == 0 {
				dispatchEntry = "serveIndexHtml"
			} else {
				dispatchEntry = "handleLen" + strconv.Itoa(assetRouteLen)
			}
		}

		dispatch[i] = handlers{
			Length:    assetRouteLen,
			Entry:     dispatchEntry,
			PrevEntry: prevEntry,
			Routes:    assetRoutes,
		}
	}

	return dispatch
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

// computeMaxLen calculates the maximum path length for dispatch array sizing.
func computeMaxLen(assets []asset) int {
	maxLen := 0
	for _, asset := range assets {
		if len(asset.RelPath) > maxLen {
			maxLen = len(asset.RelPath)
		}
	}
	return maxLen
}
