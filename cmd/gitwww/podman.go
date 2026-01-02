// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"log/slog"

	"github.com/containers/buildah/define"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities/types"
)

func (cfg *Cfg) buildPodmanImage(ctx context.Context, dir string) error {
	containerFiles := []string{cfg.Repositories[dir]["containerfile"]}
	options := define.BuildOptions{
		ContextDirectory: dir,
		Target:           cfg.Repositories[dir]["tag"],
		Args:             cfg.Repositories[dir],
		UnsetEnvs:        []string{},
		Envs:             []string{},
	}

	slog.Info("buildPodmanImage", "dir", dir, "options", options)

	buildReport, err := images.Build(ctx, containerFiles, types.BuildOptions{BuildOptions: options})
	if err != nil {
		slog.Error("buildPodmanImage", "dir", dir, "err", err)
		return err
	}

	slog.Info("buildPodmanImage", "dir", dir, "buildReport", buildReport)
	return nil
}
