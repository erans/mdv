package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erans/mdv/image"
)

// config is the file version of the flags worth remembering. Pointers say
// whether a key was present, so a missing key leaves the built-in default alone
// while `false` or `0` still count as a decision.
type config struct {
	TOC    *bool   `json:"toc"`
	Images *string `json:"images"`
	Theme  *string `json:"theme"`
	Width  *int    `json:"width"`
}

// configPath is the file to read: the -config flag, or the usual XDG location.
func configPath(explicit string) string {
	if explicit != "" {
		return expandHome(explicit)
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "mdv", "config.json")
}

// loadConfig reads the config file. A missing file is not an error: that is the
// normal case for someone who never wrote one.
func loadConfig(path string) (config, error) {
	var cfg config
	if path == "" {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Images != nil {
		if _, err := parseImages(*cfg.Images); err != nil {
			return cfg, fmt.Errorf("%s: %w", path, err)
		}
	}
	return cfg, nil
}

// applyConfig fills in whatever the user did not type on the command line.
// `set` is the set of flag names that were given explicitly, so the command line
// always wins over the file.
func applyConfig(cfg config, set map[string]bool, toc *bool, images, theme *string, width *int) {
	if !set["toc"] && cfg.TOC != nil {
		*toc = *cfg.TOC
	}
	if !set["images"] && cfg.Images != nil {
		*images = *cfg.Images
	}
	if !set["theme"] && cfg.Theme != nil {
		*theme = expandHome(*cfg.Theme)
	}
	if !set["w"] && cfg.Width != nil {
		*width = *cfg.Width
	}
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// parseImages validates a mode name from the config file with the same parser
// the flag uses.
func parseImages(mode string) (string, error) {
	if _, err := image.Parse(mode); err != nil {
		return "", err
	}
	return mode, nil
}
