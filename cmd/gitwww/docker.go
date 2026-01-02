// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/moby/go-archive"
	"github.com/moby/moby/client"
	"github.com/moby/moby/pkg/jsonmessage"
	"github.com/moby/patternmatcher/ignorefile"
	"github.com/moby/term"
)

func (cfg *Cfg) buildDockerImage(ctx context.Context, dir string) error {
	imageName := cfg.getTag(dir)

	// Configure build options
	options := build.ImageBuildOptions{
		Dockerfile:  cfg.findContainerfile(dir),
		Remove:      cfg.getRemove(dir), // if intermediate containers should be removed
		ForceRemove: cfg.getForceRemove(dir),
		NoCache:     cfg.getNoCache(dir), // disables build cache
		Tags:        []string{imageName},
		Target:      cfg.getTarget(dir), // Target specifies the build stage to target
		BuildArgs:   cfg.getDockerBuildArgs(dir),
	}
	slog.Debug("buildDockerImage", "dir", dir, "options", omitZeroEmpty(options))

	// create client that reads DOCKER_HOST, DOCKER_TLS_VERIFY...
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Warn("buildDockerImage client.NewClientWithOpts", "dir", dir, "err", err)
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// parses .dockerignore to exclude/include files
	tarOptions, err := newTarOptionsFromDockerignore(dir)
	if err != nil {
		slog.Warn("parseDockerignore", "dir", dir, "err", err)
		return fmt.Errorf("failed to create build context: %w", err)
	}

	// create build context as a tar archive
	buildCtx, err := archive.TarWithOptions(dir, tarOptions)
	if err != nil {
		slog.Warn("archive.TarWithOptions", "dir", dir, "err", err)
		return fmt.Errorf("failed to create build context: %w", err)
	}
	defer buildCtx.Close()

	// Execute the build
	resp, err := cli.ImageBuild(ctx, buildCtx, options)
	if err != nil {
		slog.Warn("buildDockerImage ImageBuild", "dir", dir, "err", err)
		return fmt.Errorf("build failed: %w", err)
	}
	defer resp.Body.Close()

	// Use the official Docker function to decode and display the stream
	termFd, isTerm := term.GetFdInfo(os.Stderr)
	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, os.Stderr, termFd, isTerm, decodeAux)
	if err != nil {
		slog.Warn("buildDockerImage", "dir", dir, "err", err)
		return err
	}

	slog.Info("✅ buildDockerImage OK", "dir", dir)

	// Create a temporary container from the image
	containerResp, err := cli.ContainerCreate(ctx, &container.Config{Image: imageName}, nil, nil, nil, "")
	if err != nil {
		slog.Warn("buildDockerImage ContainerCreate", "dir", dir, "err", err)
		return fmt.Errorf("failed to create container: %w", err)
	}
	defer func() {
		// Ensure container is removed after operation
		_ = cli.ContainerRemove(ctx, containerResp.ID, container.RemoveOptions{Force: true})
	}()

	// Copy files from container to host
	distPath := cfg.getDistPath(dir)
	reader, _, err := cli.CopyFromContainer(ctx, containerResp.ID, distPath)
	if err != nil {
		slog.Warn("buildDockerImage CopyFromContainer", "dir", dir, "err", err)
		return fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	www := cfg.getAbsWWW(dir)
	oldWWW := www + "--old"
	newWWW := www + "--new"

	// Use go-archive Untar function
	os.RemoveAll(newWWW)
	err = archive.Untar(reader, newWWW, nil)
	if err != nil {
		slog.Warn("buildDockerImage Untar", "dir", dir, "www", www, "err", err)
		return fmt.Errorf("failed to extract files: %w", err)
	}

	os.RemoveAll(oldWWW)
	os.Rename(www, oldWWW)
	os.RemoveAll(www)
	err = os.Rename(newWWW, www)
	if err != nil {
		slog.Warn("buildDockerImage Rename", "dir", dir, "newWWW", newWWW, "err", err)
		return fmt.Errorf("failed to rename www: %w", err)
	}

	return nil
}

// newTarOptionsFromDockerignore opens and reads a .dockerignore file from the specified path
// and returns an archive.TarOptions object with the parsed exclusion and inclusion patterns.
// If the .dockerignore file does not exist, it returns an TarOptions excluding some common ignored files.
func newTarOptionsFromDockerignore(dir string) (*archive.TarOptions, error) {
	// Default tar options
	tarOptions := &archive.TarOptions{
		ExcludePatterns: []string{
			".astro/", ".editorconfig", ".env*", ".git", ".gitignore",
			".idea/", ".next", ".vscode/", "coverage*", "LICENSE", "Makefile",
			"node_modules/", "npm-debug.log", "README.md",
		},
		IncludeFiles: nil,
		// IncludeFiles is not used because the patternmatcher library,
		// used internally by go-archive, will handle the distinction
		// between exclusion and inclusion based on the '!' prefix within these patterns.
		// (e.g. IncludeFiles is replaced by '!' within the ExcludePatterns entries)
	}

	// Attempt to open the .dockerignore file
	file, err := os.Open(filepath.Join(dir, ".dockerignore"))
	if err != nil {
		// If the file does not exist, return an empty TarOptions with no error.
		// This is typical behavior for .dockerignore files, where their absence
		// means no specific exclusions/inclusions are applied.
		return tarOptions, nil
	}
	defer file.Close()

	// Use ignorefile.ReadAll to parse the .dockerignore content into a slice of patterns.
	// This function handles comments, blank lines, and negation patterns (prefixed with '!')
	// as specified by the .dockerignore syntax.
	tarOptions.ExcludePatterns, err = ignorefile.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read .dockerignore patterns from %s: %w", dir, err)
	}

	return tarOptions, nil
}

func decodeAux(msg jsonmessage.JSONMessage) {
	decoded, err := msg.Aux.MarshalJSON()
	if err != nil {
		fmt.Printf("marshal err: %s\n", err)
		return
	}

	decoded = bytes.Trim(decoded, "\"")
	if len(decoded) == 0 {
		return
	}

	dst := make([]byte, 0, len(decoded)*2)
	_, err = base64.StdEncoding.Decode(dst, decoded)
	if err != nil {
		fmt.Printf("err: %v+\n", err)
		dst, err = base64.StdEncoding.DecodeString(string(decoded))
		if err != nil {
			fmt.Printf("aux: %q\n", decoded)
			fmt.Printf("err: %v+\n", err)
		}
	}
	fmt.Printf("base64: %s\n", dst)
}

func omitZeroEmpty(v any) any {
	b, _ := json.Marshal(v)
	json.Unmarshal(b, &v)
	return prune(v)
}

func prune(m any) any {
	switch x := m.(type) {
	case map[string]any:
		if len(x) == 0 {
			return nil
		}
		out := make(map[string]any, len(x))
		for k, v := range x {
			if pv := prune(v); pv != nil {
				out[k] = pv
			}
		}
		return out
	case []any:
		if len(x) == 0 {
			return nil
		}
		arr := make([]any, 0, len(x))
		for _, e := range x {
			if pv := prune(e); pv != nil {
				arr = append(arr, pv)
			}
		}
		return arr
	case string:
		if x == "" {
			return nil
		}
	case bool:
		if !x {
			return nil
		}
	case float32, float64:
		if x.(float64) == 0 {
			return nil
		}
	case complex64, complex128:
		if x.(complex128) == 0 {
			return nil
		}
	case uint, uint8, uint16, uint32, uint64, uintptr:
		if x.(uint64) == 0 {
			return nil
		}
	case int, int8, int16, int32, int64:
		if x.(int64) == 0 {
			return nil
		}
	}
	return m
}
