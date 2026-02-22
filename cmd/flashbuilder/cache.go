// Package: main
// Purpose: Cache management, budget allocation, cache cleaning
// File: cache.go

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// FileInfo holds file information for cache management
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// ensureCacheDir ensures cache directory exists
func ensureCacheDir(cacheDir string) error {
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return fmt.Errorf("E099: Failed to create cache directory: %v", err)
	}
	return nil
}

// cleanCache maintains cache size within configured limits
// Removes oldest files when cache exceeds maxSize
func cleanCache(cacheDir string, maxSize int64) error {
	var totalSize int64
	var files []FileInfo

	err := filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files = append(files, FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		totalSize += info.Size()
		return nil
	})
	if err != nil {
		return fmt.Errorf("E099: Failed to walk cache directory: %v", err)
	}

	// If total size exceeds max, delete oldest files
	if totalSize > maxSize {
		// Sort by modification time (oldest first)
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.Before(files[j].ModTime)
		})

		// Delete oldest files until total size is under max
		for _, file := range files {
			if totalSize <= maxSize {
				break
			}
			os.Remove(file.Path)
			totalSize -= file.Size
		}
	}

	return nil
}

// allocateBudget determines which assets are eligible for embedding
// Assets are sorted by size (smallest first) and embedded until budget exhausted
func allocateBudget(assets []asset, budget int64) []asset {
	// Sort assets by size (smallest first for embedding priority)
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].Size < assets[j].Size
	})

	var totalSize int64
	for i := range assets {
		if totalSize+assets[i].Size <= budget {
			assets[i].EmbedEligible = true
			totalSize += assets[i].Size
		} else {
			assets[i].EmbedEligible = false
		}
	}

	return assets
}
