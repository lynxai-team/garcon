// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func main() {
	cfg, err := getCfg()
	if err != nil {
		os.Exit(1)
	}
	if cfg == nil {
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		for dir, params := range cfg.reposSeq() {
			repo := cfg.shouldDeploy(dir, params)
			if repo != nil {
				cfg.buildDeploy(ctx, repo, dir, params)
			}
		}

		time.Sleep(time.Duration(cfg.Sleep) * time.Second)
	}
}

func (cfg *Cfg) reposSeq() iter.Seq2[string, map[string]string] {
	return func(yield func(string, map[string]string) bool) {
		if !filepath.IsAbs(cfg.Repos) || !filepath.IsAbs(cfg.WWW) {
			cfg = cfg.clone()
			var err error
			cfg.Repos, err = filepath.Abs(cfg.Repos)
			if err != nil {
				slog.Error("filepath.Abs(repos)", "err", err)
				return
			}
			cfg.WWW, err = filepath.Abs(cfg.WWW)
			if err != nil {
				slog.Error("filepath.Abs(www)", "err", err)
				return
			}
		}

		for dir, repo := range cfg.absRepositories() {
			file := cfg.findContainerfile(repo)
			if file == "" {
				continue
			}

			params := cfg.Repositories[repo]
			if params == nil {
				params = make(map[string]string, 3)
			}
			params["containerfile"] = file
			params["www"] = cfg.getAbsWWW(repo)
			params["tag"] = cfg.getTag(repo)

			if !yield(dir, params) {
				return
			}
		}
	}
}
