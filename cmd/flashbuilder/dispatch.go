// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"path/filepath"
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

// buildGetRoutesByLength groups routes by length, sorted by frequency score.
func buildGetRoutesByLength(assets []asset, size int) [][]asset {
	routesByLen := make([][]asset, size)

	// 1. group routes by length
	for _, a := range assets {
		routeLen := len(a.RelPath) // relative path without leading slash
		routesByLen[routeLen] = append(routesByLen[routeLen], a)
	}

	// 2. sort by frequency score within each length group
	for routeLen, assets := range routesByLen {
		sort.Slice(assets, func(i, j int) bool {
			return assets[i].Frequency > assets[j].Frequency
		})
		routesByLen[routeLen] = assets
	}

	return routesByLen
}

// buildRoutesByLength groups routes by length, sorted by frequency score.
func buildPostRoutesByLength(assets []asset, size int) [][]asset {
	routesByLen := make([][]asset, size)

	// group routes by length
	existing := existing{}
	for _, a := range assets {
		for route := range a.Form {
			if route != "" && route[0] == '/' {
				route = route[1:] // drop leading slash
			} else {
				// sanitize relative path
				dir := filepath.Dir(a.RelPath)
				route, err := filepath.Rel(dir, route)
				if err != nil {
					fmt.Println("WARN cannot deduce relative path ", dir, route, err)
				}
			}

			routeLen := len(route)

			if slices.ContainsFunc(routesByLen[routeLen], func(a asset) bool { return a.RelPath == route }) {
				continue
			}

			modifiedAsset := a
			modifiedAsset.Identifier = existing.generateIdentifier(route)
			modifiedAsset.RelPath = route
			routesByLen[routeLen] = append(routesByLen[routeLen], modifiedAsset)
		}
	}
	// no idea how to sort the POST endpoints
	return routesByLen
}

// buildGet generates the dispatch array for the GET handlers.
// We use `index = assetRouteLen + 1` to eliminate the runtime slash removal.
func buildGet(assets []asset, maxLen int) []handlers {
	routesByLen := buildGetRoutesByLength(assets, maxLen+1)
	get := make([]handlers, maxLen+2)
	function := "notFound"

	for i := range get {
		// index (of the get array) is the length of the request path (including leading slash)
		// the asset routes are relative paths (no leading slash)
		routeLen := max(0, i-1) // subtract the leading slash
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
// We use `index = assetRouteLen + 1` to eliminate the runtime slash removal.
func buildPost(assets []asset, maxLen int) []handlers {
	routesByLen := buildPostRoutesByLength(assets, maxLen+1)
	post := make([]handlers, maxLen+2)
	atLeastOnePost := false

	for i := range post {
		// index (of the get array) is the length of the request path (including leading slash)
		// the asset routes are relative paths (no leading slash)
		routeLen := max(0, i-1) // subtract the leading slash
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
