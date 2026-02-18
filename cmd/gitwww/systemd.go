// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed gitwww.service
var data []byte

func (cfg *Cfg) writeSystemdService() (string, error) {
	end := len(defaultCfgDir) - 1
	if cfg.Repos != defaultCfgDir[:end] {
		reposSlash := append([]byte(cfg.Repos), '/')
		data = bytes.ReplaceAll(data, []byte(defaultCfgDir), reposSlash)
	}

	oldCFG := append([]byte(GITWWW_CFG), '=')
	oldWWW := append([]byte(GITWWW_WWW), '=')
	oldLOG := append([]byte(GITWWW_LOG), '=')

	newCFG := append(oldCFG, []byte(cfg.Repos)...)
	newWWW := append(oldWWW, []byte(cfg.WWW)...)
	newLOG := append(oldLOG, []byte(cfg.LogLevel)...)

	data = bytes.Replace(data, oldCFG, newCFG, 1)
	data = bytes.Replace(data, oldWWW, newWWW, 1)
	data = bytes.Replace(data, oldLOG, newLOG, 1)

	path := cfg.Path
	if fileExists(cfg.Path) || !directoryExists(cfg.Path) && filepath.Base(cfg.Path) == defaultCfgName {
		path = filepath.Dir(cfg.Path)
	}
	err := os.MkdirAll(path, 0o700)
	path = filepath.Join(path, gitwwwService)

	if err != nil {
		return path, err
	}

	return path, os.WriteFile(path, data, 0o600)
}
