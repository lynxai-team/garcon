// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// Log messages.
func logMessage(msg string) {
	log.Printf("\033[34m%s\033[m \033[32m%s\033[m", time.Now().Format("15:04"), msg)
}

// Log error messages.
func logError(msg string) {
	log.Printf("\033[34m%s\033[m \033[31m%s\033[m", time.Now().Format("15:04"), msg)
}

// buildDeploy retrieves the new Git commits,
// builds using the provided Containerfile,
// and copies the files from the container image to the www directory.
func (cfg *Cfg) buildDeploy(ctx context.Context, repo *git.Repository, dir string, params map[string]string) {
	err := gitPull(repo, params)
	if err != nil {
		logError("KO git pull. Local changes might exist.")
		return
	}

	engines, found := params["engine"]
	if !found {
		engines = cfg.Engine
	}

	for engine := range strings.SplitSeq(engines, ",") {
		switch engine {
		case "docker":
			err = cfg.buildDockerImage(ctx, dir)
		case "podman":
			err = cfg.buildPodmanImage(ctx, dir)
		default:
			logError("Unexpected engine=" + engine)
		}
		if err == nil {
			break
		}
	}

	if err != nil {
		logError("KO commit")
		return
	}
}

// gitPull pulls changes from the remote repository (or performs a `git reset --hard`).
func gitPull(repo *git.Repository, params map[string]string) error {
	worktree, err := repo.Worktree()
	if err != nil {
		return err
	}

	branch, found := params["branch"]
	if !found {
		branch = "origin/main"
	}
	remote, _, found := strings.Cut(branch, "/")
	if !found {
		remote = "origin"
	}

	err = worktree.Pull(&git.PullOptions{
		RemoteName:        remote,
		Force:             true,
		RemoteURL:         "",
		ReferenceName:     "",
		SingleBranch:      false,
		Depth:             0,
		Auth:              nil,
		RecurseSubmodules: 0,
		Progress:          nil,
		InsecureSkipTLS:   false,
		ClientCert:        nil,
		ClientKey:         nil,
		CABundle:          nil,
		ProxyOptions:      transport.ProxyOptions{},
	})

	if err == nil || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}

	// If pulling fails, reset to origin/main
	return worktree.Reset(&git.ResetOptions{
		Mode:   git.HardReset,
		Commit: plumbing.NewHash(branch),
		Files:  nil,
	})
}

func (cfg *Cfg) getTarget(dir string) string {
	return cfg.Repositories[dir]["target"]
}

func (cfg *Cfg) getTag(dir string) string {
	tag := cfg.Repositories[dir]["tag"]
	if tag != "" {
		return tag
	}
	return filepath.Base(dir)
}

func (cfg *Cfg) getRemove(dir string) bool {
	rm := cfg.Repositories[dir]["remove"]
	return rm == "1" || strings.Contains(strings.ToLower(rm), "true")
}

func (cfg *Cfg) getForceRemove(dir string) bool {
	rm := cfg.Repositories[dir]["force-remove"]
	return rm == "1" || strings.Contains(strings.ToLower(rm), "true")
}

func (cfg *Cfg) getNoCache(dir string) bool {
	rm := cfg.Repositories[dir]["no-cache"]
	return rm == "1" || strings.Contains(strings.ToLower(rm), "true")
}

func (cfg *Cfg) getDockerBuildArgs(dir string) map[string]*string {
	params := cfg.Repositories[dir]
	if len(params) == 0 {
		return nil
	}

	args := make(map[string]*string, len(params))
	for k, v := range params {
		args[k] = &v
	}
	return args
}

func (cfg *Cfg) getDistPath(dir string) string {
	dist := cfg.Repositories[dir]["dist-path"]
	if dist == "" {
		return "/dist"
	}
	return dist
}

// findContainerfile searches for Containerfile, Dockerfile...
func (cfg *Cfg) findContainerfile(dir string) string {
	abs := cfg.Abs(dir)
	if abs == "" {
		return ""
	}

	name := cfg.Repositories[dir]["containerfile"]
	if name != "" {
		file := name
		if !filepath.IsAbs(name) {
			file = filepath.Join(abs, name)
		} else {
			slog.Warn("[containerfile] should a relative path", "dir", abs, "file", file)
		}
		if !fileExists(file) {
			slog.Warn("[containerfile] does not exist", "dir", abs, "file", file)
			name = ""
		}
		return name
	}

	for _, name := range []string{"Containerfile", "Dockerfile"} {
		file := filepath.Join(abs, name)
		if fileExists(file) {
			return name
		}
	}

	// Define the regex pattern for matching Dockerfile or Containerfile
	pattern := regexp.MustCompile(`.*(Contain|Dock)erfile.*`)
	// Traverse the directory and its subdirectories
	_ = filepath.Walk(
		abs,
		func(_ string, info os.FileInfo, _ error) error {
			// Check if the file matches the pattern
			if !info.IsDir() && pattern.MatchString(info.Name()) {
				name = info.Name()
				return filepath.SkipDir // Stop walking the directory
			}
			return nil
		},
	)
	return name
}
