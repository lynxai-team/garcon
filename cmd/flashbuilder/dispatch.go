// Package: main
// Purpose: Dispatch array generation, routing, path sanitization
// File: dispatch.go

package main

import (
	"fmt"
	"sort"
	"strings"
)

// RouteData represents a route for switch statements
type RouteData struct {
	Path       string // Sanitized path for switch case
	Identifier string // Go identifier for handler function
	Frequency  int    // Request frequency score (for ordering)
}

// DispatchEntry represents a dispatch array entry
type DispatchEntry struct {
	Index   int
	Handler string
	Routes  []RouteData
}

// DispatchData holds dispatch array data for template rendering
type DispatchData struct {
	HTTP     []DispatchEntry
	HTTPS    []DispatchEntry
	MaxLen   int
}

// sanitizePath converts a path to a valid Go string literal for switch cases
func sanitizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")

	var result strings.Builder
	for _, r := range path {
		switch r {
		case '\\':
			result.WriteString("\\\\")
		case '"':
			result.WriteString("\\\"")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// buildDispatch generates dispatch arrays for HTTP and HTTPS
// Dispatch index = route length + 1 (eliminates runtime slash removal)
func buildDispatch(assets []asset, maxLen int) (httpDispatch, httpsDispatch []DispatchEntry) {
	// Group routes by length (asset route length = dispatch index - 1)
	routesByLength := make(map[int][]RouteData)

	for _, asset := range assets {
		if asset.IsDuplicate {
			continue
		}
		routeLength := len(asset.RelPath)
		dispatchIndex := routeLength + 1

		route := RouteData{
			Path:       sanitizePath(asset.RelPath),
			Identifier: asset.Identifier,
			Frequency:  asset.FrequencyScore,
		}

		routesByLength[dispatchIndex] = append(routesByLength[dispatchIndex], route)
	}

	// Sort routes by frequency score within each length group
	for _, routes := range routesByLength {
		sort.Slice(routes, func(i, j int) bool {
			return routes[i].Frequency > routes[j].Frequency
		})
	}

	httpDispatch = make([]DispatchEntry, maxLen+2)
	httpsDispatch = make([]DispatchEntry, maxLen+2)

	// Index 0 and 1: Root handlers
	rootHandler := "serveRootIndex"
	if hasRootIndex(assets) {
		httpDispatch[0] = DispatchEntry{Index: 0, Handler: rootHandler}
		httpDispatch[1] = DispatchEntry{Index: 1, Handler: rootHandler}
		httpsDispatch[0] = DispatchEntry{Index: 0, Handler: rootHandler}
		httpsDispatch[1] = DispatchEntry{Index: 1, Handler: rootHandler}
	} else {
		httpDispatch[0] = DispatchEntry{Index: 0, Handler: "http.NotFound"}
		httpDispatch[1] = DispatchEntry{Index: 1, Handler: "http.NotFound"}
		httpsDispatch[0] = DispatchEntry{Index: 0, Handler: "http.NotFound"}
		httpsDispatch[1] = DispatchEntry{Index: 1, Handler: "http.NotFound"}
	}

	for index := 2; index <= maxLen+1; index++ {
		routes := routesByLength[index]
		if len(routes) == 0 {
			// No routes at this length, fallback to previous
			fallback := index - 1
			for fallback >= 0 && httpDispatch[fallback].Handler == "" {
				fallback--
			}
			if fallback < 0 {
				httpDispatch[index] = DispatchEntry{Index: index, Handler: "http.NotFound"}
				httpsDispatch[index] = DispatchEntry{Index: index, Handler: "http.NotFound"}
			} else {
				httpDispatch[index] = httpDispatch[fallback]
				httpsDispatch[index] = httpsDispatch[fallback]
			}
			continue
		}

		handlerName := fmt.Sprintf("handleLen%d", index)
		httpDispatch[index] = DispatchEntry{
			Index:   index,
			Handler: handlerName + "HTTP",
			Routes:  routes,
		}
		httpsDispatch[index] = DispatchEntry{
			Index:   index,
			Handler: handlerName + "HTTPS",
			Routes:  routes,
		}
	}

	return httpDispatch, httpsDispatch
}

// hasRootIndex checks if any asset is the root index file
func hasRootIndex(assets []asset) bool {
	for _, asset := range assets {
		if asset.RelPath == "index.html" || asset.RelPath == "" {
			return true
		}
	}
	return false
}

// computeMaxLen calculates the maximum path length for dispatch array sizing
func computeMaxLen(assets []asset) int {
	maxLen := 0
	for _, asset := range assets {
		if len(asset.RelPath) > maxLen {
			maxLen = len(asset.RelPath)
		}
	}
	return maxLen
}
