// Copyright 2021 The contributors of Garcon.
// This file is part of Garcon, an automatic static-site builder, API server, middlewares and messy functions.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/goccy/go-yaml"
	"github.com/pelletier/go-toml/v2"
)

type Cfg struct {
	Repositories map[string]map[string]string `toml:"-"      yaml:"-"      comment:"Git repos to watch and their build arguments"`
	Repos        string                       `toml:"repos"  yaml:"repos"  comment:"\ndirectory containing the repositories to build/deploy (default /var/opt/garcon)"`
	WWW          string                       `toml:"www"    yaml:"www"    comment:"\nfinal destination of the deployed static web file (default /var/opt/www)"`
	Engine       string                       `toml:"engine" yaml:"engine" comment:"\none or two container management tools (separated by a comma) among docker and podman (default docker)"`
	LogLevel     string                       `toml:"log"    yaml:"log"    comment:"\nlog verbosity level can be DEBUG, INFO, WARN and ERROR (default INFO)"`
	Sleep        int                          `toml:"sleep"  yaml:"sleep"  comment:"\nseconds before checking new Git commits (default 10 seconds)"`
}

const (
	defaultCfgDir  = "/var/opt/gitwww/"
	defaultCfgName = "gitwww.ini"
	defaultCfgPath = defaultCfgDir + defaultCfgName

	GITWWW_CFG = "GITWWW_CFG"
	GITWWW_LOG = "GITWWW_LOG"
)

func (cfg *Cfg) clone() *Cfg {
	c2 := *cfg
	return &c2
}

func getCfg() (*Cfg, error) {
	path := flag.String("c", defaultCfgPath, "Configuration (file or directory), take precedence on "+GITWWW_CFG)
	debug := flag.Bool("d", false, "debug mode, same as "+GITWWW_LOG+"=DEBUG")
	quiet := flag.Bool("q", false, "quiet mode, same as "+GITWWW_LOG+"=WARN")
	write := flag.Bool("w", false, "write the configuration file")
	absolute := flag.Bool("ww", false, "overwrite an explicit version of the configuration file using absolute paths")
	simplify := flag.Bool("www", false, "overwrite a simplified version of the configuration file")
	clean := flag.Bool("wwww", false, "overwrite a very simplified version of the configuration file: use the minimum required repo parameters")
	flag.Parse()

	if *clean {
		*simplify = true
	}
	if *simplify || *absolute {
		*write = true
	}

	if *path == defaultCfgPath {
		*path = os.Getenv(GITWWW_CFG)
	}
	if *path == "" {
		*path = defaultCfgPath
	}

	if directoryExists(*path) {
		*path = filepath.Join(*path, defaultCfgName)
	}

	cfg := &Cfg{ // default values
		Repos:  filepath.Join(filepath.Dir(*path), "repos"),
		WWW:    "/var/opt/www",
		Engine: "docker", // use "docker,podman" to try docker, then podman if docker is not working
		Sleep:  10,       // 10 seconds
	}

	data, err := os.ReadFile(*path)
	if err != nil && *path == defaultCfgPath && !directoryExists(defaultCfgDir) {
		slog.Info("Use local config because no default configuration found", "dir", defaultCfgDir, "err", err)
		*path = defaultCfgName
		data, err = os.ReadFile(*path)
		if directoryExists("repos") {
			cfg.Repos = "repos"
		}
		if !directoryExists(cfg.WWW) {
			cfg.WWW = "www"
		}
	}
	if err != nil || len(data) == 0 {
		slog.Info("Use default settings because no configuration file (or empty)", "path", *path, "err", err)
		if !directoryExists(cfg.Repos) {
			cfg.Repos = filepath.Dir(cfg.Repos)
		}
		if !*absolute {
			*simplify = true
		}
	} else {
		pos := bytes.IndexByte(data, '[')
		if pos < 0 {
			pos = len(data)
		}

		err = toml.Unmarshal(data[:pos], cfg)
		if err != nil {
			slog.Error("Failed to parse #1", "path", *path, "err", err, "cfgData", string(data[:200]))
			return nil, err
		}
		if pos < len(data) {
			var tables map[string]map[string]string
			err = toml.Unmarshal(data[pos:], &tables)
			if err == nil {
				cfg.Repositories = tables
			} else {
				fmt.Println("Failed to parse #2", "path", *path, "err", err, "cfgData", string(data[:200]))
			}
		}
	}

	lvl := cfg.getLevel(*debug, *quiet)
	cfg.LogLevel = lvl.String()
	slog.SetLogLoggerLevel(lvl)

	sanitized := cfg.clone()
	err = sanitized.sanitize()
	if err != nil {
		slog.Error("Failed to deduce absolute paths", "err", err)
		data, err = yaml.Marshal(cfg)
		if err != nil {
			slog.Error("Failed to yaml.Marshal", "err", err, "cfg", cfg)
			return nil, err
		}
		os.Stdout.WriteString("-------------------------------------------\n")
		os.Stdout.Write(data)
		os.Stdout.WriteString("-------------------------------------------\n")
		return nil, err
	}

	if !*quiet {
		data, err = yaml.Marshal(sanitized)
		if err != nil {
			slog.Error("Failed to yaml.Marshal", "err", err, "sanitized", sanitized)
			return nil, err
		}
		os.Stdout.WriteString("-------------------------------------------\n")
		os.Stdout.Write(data)
		os.Stdout.WriteString("-------------------------------------------\n")
		if len(sanitized.Repositories) > 0 {
			data, err = yaml.Marshal(sanitized.Repositories)
			if err != nil {
				slog.Error("Failed to yaml.Marshal", "err", err, "sanitized.Repositories", sanitized.Repositories)
				return nil, err
			}
			os.Stdout.Write(data)
			os.Stdout.WriteString("-------------------------------------------\n")
		}
	}

	if *write {
		if *simplify || *absolute {
			cfg = sanitized
		}
		if *simplify {
			err = cfg.simplify(*clean)
			if err != nil {
				slog.Error("Cannot simplify", "err", err, "sanitized", sanitized)
				return nil, err
			}
		}
		data, err = toml.Marshal(cfg)
		if err != nil {
			slog.Error("Failed to toml.Marshal", "err", err, "cfg", cfg)
			return nil, err
		}
		f, err := os.Create(*path)
		if err != nil {
			slog.Error("Cannot create", "file", *path, "err", err)
			return nil, err
		}
		defer f.Close()
		_, err = f.Write(data)
		if err != nil {
			slog.Error("Cannot write #1", "file", *path, "err", err)
		}
		if len(cfg.Repositories) > 0 {
			data, err = toml.Marshal(cfg.Repositories)
			if err != nil {
				slog.Error("Failed to toml.Marshal", "err", err, "cfg", cfg)
				return nil, err
			}
			_, err = f.WriteString("\n\n# Git repos to watch new commits and their build arguments\n\n")
			if err != nil {
				slog.Error("Cannot write #2", "file", *path, "err", err)
			}
			_, err = f.Write(data)
			if err != nil {
				slog.Error("Cannot write #3", "file", *path, "err", err)
			}
		}
		slog.Info("Flag -w (or -ww or -www) => exit after writing", "file", *path)
		return nil, nil
	}

	return cfg, nil
}

func (cfg *Cfg) getLevel(debug, quiet bool) slog.Level {
	switch {
	case debug:
		return slog.LevelDebug
	case quiet:
		return slog.LevelWarn
	}

	txt, found := syscall.Getenv(GITWWW_LOG)
	if !found {
		txt = cfg.LogLevel
	}

	if txt == "" {
		return slog.LevelInfo
	}

	var lvl slog.Level

	_ = lvl.UnmarshalText([]byte(txt))

	return lvl
}

// simplify enforces relative paths and drop parameters that can be easily deduced.
func (cfg *Cfg) simplify(clean bool) error {
	newRepos := make(map[string]map[string]string, len(cfg.Repositories))
	for dir, params := range cfg.Repositories {
		base := filepath.Base(dir)
		if params != nil {
			www, found := params["www"]
			if found {
				rel, found := strings.CutPrefix(www, cfg.WWW)
				if found && rel != "" {
					www = rel[1:] // drop leading os.PathSeparator
					params["www"] = www
				}
				if clean && (www == base || www == "") {
					delete(params, "www")
				}
			}

			file, found := params["containerfile"]
			if found {
				rel, found := strings.CutPrefix(file, dir)
				if found && rel != "" {
					file = rel[1:] // drop leading os.PathSeparator
					params["containerfile"] = file
				}
				if clean && (file == "Containerfile" || file == "Dockerfile" || file == "") {
					delete(params, "containerfile")
				}
			}

			tag := params["tag"]
			if clean && tag == base {
				delete(params, "tag")
			}
		}

		rel, _ := strings.CutPrefix(dir, cfg.Repos)
		if rel != "" {
			if clean && len(params) == 0 && directoryExists(dir) {
				continue
			}
			dir = rel[1:] // drop leading os.PathSeparator
		}
		newRepos[dir] = params
	}
	cfg.Repositories = newRepos

	pwd, err := filepath.Abs(".")
	if err != nil {
		return err
	}

	rel, found := strings.CutPrefix(cfg.Repos, pwd)
	if found && rel != "" {
		cfg.Repos = rel[1:] // drop leading os.PathSeparator
	}

	rel, found = strings.CutPrefix(cfg.WWW, pwd)
	if found && rel != "" {
		cfg.WWW = rel[1:] // drop leading os.PathSeparator
	}

	return nil
}

// sanitize enforces absolute paths.
func (cfg *Cfg) sanitize() error {
	var err error
	cfg.Repos, err = filepath.Abs(cfg.Repos)
	if err != nil {
		return err
	}

	cfg.WWW, err = filepath.Abs(cfg.WWW)
	if err != nil {
		return err
	}

	absToRepo := cfg.absRepositories()
	newRepos := make(map[string]map[string]string, len(absToRepo))
	for abs, repo := range absToRepo {
		params := cfg.Repositories[repo]
		if params == nil {
			params = make(map[string]string, 3)
		}
		params["containerfile"] = cfg.findContainerfile(repo)
		params["www"] = cfg.getAbsWWW(repo)
		params["tag"] = cfg.getTag(repo)
		newRepos[abs] = params
	}

	cfg.Repositories = newRepos
	return nil
}

// absRepositories returns the absolute path of the repo directories.
func (cfg *Cfg) absRepositories() map[string]string {
	directories, err := cfg.subDirectories()
	if err != nil {
		directories = make(map[string]string, len(cfg.Repositories))
	}
	for repo := range cfg.Repositories {
		abs := cfg.Abs(repo)
		if abs == "" {
			slog.Debug("skip no exist", "repo", repo)
			continue
		}
		file := cfg.findContainerfile(repo)
		if file == "" {
			slog.Debug("skip no Containerfile/Dockerfile", "repo", repo, "abs", abs)
			continue
		}
		slog.Debug("add", "repo", repo, "abs", abs)
		directories[abs] = repo
	}
	return directories
}

// subDirectories lists the first-level sub-directories under the given root.
// It only lists immediate children and returns an error if anything goes wrong.
func (cfg *Cfg) subDirectories() (map[string]string, error) {
	root, err := os.Open(cfg.Repos)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	// list the files of the directory
	files, err := root.Readdir(-1)
	if err != nil {
		return nil, err
	}

	// collect only the directories
	directories := make(map[string]string, len(files))
	for _, dir := range files {
		if !dir.IsDir() {
			continue
		}
		path := filepath.Join(cfg.Repos, dir.Name())
		abs, err := filepath.Abs(path)
		if err != nil {
			slog.Warn("Skip", "dir", dir, "filepath.Abs err", err)
			continue
		}
		file := cfg.findContainerfile(abs)
		if file == "" {
			slog.Info("Skip no Containerfile/Dockerfile", "abs", abs)
			continue
		}
		slog.Debug("add sub", "dir", dir.Name(), "path", path, "abs", abs)
		directories[abs] = abs
	}

	return directories, nil
}

func (cfg *Cfg) getAbsWWW(dir string) string {
	www := cfg.Repositories[dir]["www"] // folder where static files will be copied
	if www == "" {
		www = filepath.Base(dir)
	} else if filepath.IsAbs(www) {
		return www
	}
	return filepath.Join(cfg.WWW, www)
}
