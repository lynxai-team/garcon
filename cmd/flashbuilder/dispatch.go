// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
)

// handlers is used to render the handlers and get array.
type handlers struct {
	PrevEntry string
	Entry     string
	Routes    []asset
	Length    int
}

// buildGetRoutesByLength groups routes by length, sorted by frequency score.
func buildGetRoutesByLength(assets []asset, size int) [][]asset {
	routesByLen := make([][]asset, size+1) // allocate size+1 to avoid panic

	// group routes by length
	for _, a := range assets {
		routeLen := len(a.Path)
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
		for route := range a.Form {
			if route != "" && route[0] == '/' {
				route = route[1:] // drop leading slash
			} else {
				// sanitize relative path
				dir := path.Dir(a.Path)
				var err error
				route, err = filepath.Rel(dir, route)
				if err != nil {
					fmt.Println("WARN cannot deduce relative path ", dir, route, err)
				}
			}

			routeLen := len(route)

			if routeLen < len(routesByLen) &&
				slices.ContainsFunc(routesByLen[routeLen], func(a asset) bool { return route == a.Path }) {
				continue
			}

			modifiedAsset := a
			modifiedAsset.Identifier = exist.generateIdentifier(route)
			modifiedAsset.Path = route
			// Ensure routesByLen is large enough
			if routeLen >= len(routesByLen) {
				newRoutes := make([][]asset, routeLen+2)
				copy(newRoutes, routesByLen)
				routesByLen = newRoutes
			}
			routesByLen[routeLen] = append(routesByLen[routeLen], modifiedAsset)
		}
	}
	// No sorting logic for POST endpoints yet, leaving as is.
	return routesByLen
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

func addShortcutPaths(assets []asset) []asset {
	paths := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		paths[asset.Path] = struct{}{}
	}

	shortcuts := make([]asset, 0, len(assets))
	for a := range slices.Values(assets) {
		shortPath := generateShortcut(a.Path)
		_, found := paths[shortPath]
		if !found {
			a.IsShortcut = true
			a.Path = ""
			a.Path = shortPath
			shortcuts = append(shortcuts, a)
			paths[shortPath] = struct{}{}
		}
	}
	return append(assets, shortcuts...)
}

func computeMaxLen(assets []asset) int {
	maxLen := 0
	for _, asset := range assets {
		if len(asset.Path) > maxLen {
			maxLen = len(asset.Path)
		}
	}
	return maxLen
}
