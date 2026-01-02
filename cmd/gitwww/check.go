// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// directoryExists checks if a directory exists.
func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false // does not exist
	}
	return info.IsDir() // true if it is a directory
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false // does not exist
	}
	return !info.IsDir()
}

// Abs returns the absolute directory if repo ready, else an empty string.
func (cfg *Cfg) Abs(repo string) string {
	enable, found := cfg.Repositories[repo]["enable"]
	if found && strings.EqualFold(enable, "false") {
		slog.Info("Skip", "repo", repo, "enable", enable)
		return ""
	}

	clone := cfg.Repositories[repo]["clone"]

	if filepath.IsAbs(repo) {
		if clone != "" || directoryExists(repo) {
			return repo
		}
		slog.Info("Skip because absolute path does not exist", "repo", repo)
		return ""
	}

	dir := filepath.Join(cfg.Repos, repo)
	if filepath.IsAbs(dir) {
		if clone != "" || directoryExists(dir) {
			return dir
		}
		slog.Info("Skip because absolute path does not exist", "dir", dir)
		return ""
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		slog.Warn("Skip", "dir", dir, "filepath.Abs err", err)
		return ""
	}
	if clone != "" || directoryExists(abs) {
		return abs
	}
	slog.Info("Skip because absolute path does not exist", "abs", abs)
	return ""
}

func (cfg *Cfg) shouldDeploy(abs string, params map[string]string) *git.Repository {
	repo, err := git.PlainOpen(abs)
	if err != nil {
		slog.Warn("Cannot git.PlainOpen", "dir", abs, "err", err)
		return nil
	}

	if !directoryExists(params["www"]) {
		slog.Info("shouldDeploy because no dir", "www", params["www"])
		return repo
	}

	branch, found := params["branch"]
	if !found {
		branch = "origin/main"
	}
	ref := plumbing.ReferenceName("refs/remotes/" + branch)

	remoteRef, err := repo.Reference(ref, true)
	if err != nil {
		slog.Warn("Cannot repo.Reference", "dir", abs, "ref", ref, "err", err)
		return nil
	}

	localRef, err := repo.Head()
	if err != nil {
		slog.Warn("Cannot repo.Head", "dir", abs, "err", err)
		return nil
	}

	if remoteRef.Hash() == localRef.Hash() {
		return nil // same commit
	}

	slog.Info("shouldDeploy because new commit")
	logHistory(repo, remoteRef.Hash(), localRef.Hash())
	return repo
}

func logHistory(repo *git.Repository, headHash, stopHash plumbing.Hash) {
	// Get the commit history starting from remote HEAD
	cIter, err := repo.Log(&git.LogOptions{From: headHash})
	if err != nil {
		slog.Error("Failed to get commit history", "err", err)
		os.Exit(1)
	}

	// Iterate over the last 10 commits
	count := 0
	err = cIter.ForEach(func(commit *object.Commit) error {
		// Get the patch for the commit
		patch, err := getCommitPatch(commit)
		if err != nil {
			slog.Warn("Failed to get commit patch", "commit_hash", commit.Hash.String(), "err", err)
		}

		slog.Info("Commit",
			slog.String("hash", commit.Hash.String()[:5]),
			slog.String("name", commit.Author.Name),
			slog.String("email", commit.Author.Email),
			slog.Time("when", commit.Author.When),
			slog.Int("lines", strings.Count(patch, "\n")),
			// slog.String("committer_name", commit.Committer.Name),
			// slog.String("committer_email", commit.Committer.Email),
			// slog.Time("committer_when", commit.Committer.When),
			slog.String("message", strings.TrimSpace(commit.Message)),
			// slog.String("tree_hash", commit.TreeHash.String()),
			// slog.Int("num_parents", len(commit.ParentHashes)),
			// slog.String("patch", patch), // Log the patch
		)
		count++

		if count > 9 {
			return errors.New("stop iteration") // Stop after 10 commits
		}
		if commit.Hash == stopHash {
			return errors.New("stop iteration") // Stop after 10 commits
		}

		return nil
	})

	if err != nil && err.Error() != "stop iteration" {
		slog.Error("Error iterating commits", "err", err)
	}
}

// getCommitPatch gets the patch (diff) for a commit.
func getCommitPatch(commit *object.Commit) (string, error) {
	// If it's the initial commit, there's no parent to diff against
	if commit.NumParents() == 0 {
		return "", nil
	}

	parent, err := commit.Parents().Next()
	if err != nil {
		return "", fmt.Errorf("failed to get parent commit: %w", err)
	}

	patch, err := parent.Patch(commit)
	if err != nil {
		return "", fmt.Errorf("failed to generate patch: %w", err)
	}

	return patch.String(), nil
}
