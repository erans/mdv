package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, `{"toc": true, "images": "sixel", "width": 100, "theme": "~/t.toml"}`)
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TOC == nil || !*cfg.TOC {
		t.Errorf("toc = %v", cfg.TOC)
	}
	if cfg.Images == nil || *cfg.Images != "sixel" {
		t.Errorf("images = %v", cfg.Images)
	}
	if cfg.Width == nil || *cfg.Width != 100 {
		t.Errorf("width = %v", cfg.Width)
	}
}

func TestMissingConfigIsFine(t *testing.T) {
	cfg, err := loadConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("a missing config should not be an error: %v", err)
	}
	if cfg.TOC != nil || cfg.Images != nil || cfg.Width != nil || cfg.Theme != nil {
		t.Errorf("missing config produced %+v", cfg)
	}
}

func TestBrokenConfigReports(t *testing.T) {
	if _, err := loadConfig(writeConfig(t, `{"toc": `)); err == nil {
		t.Error("invalid JSON should be reported")
	}
	if _, err := loadConfig(writeConfig(t, `{"images": "telepathy"}`)); err == nil {
		t.Error("an unknown image mode should be reported")
	}
}

func TestApplyConfigLetsTheCommandLineWin(t *testing.T) {
	yes := true
	images, theme, width := "sixel", "/from/config.toml", 70
	cfg := config{TOC: &yes, Images: &images, Theme: &theme, Width: &width}

	toc := false
	img, th, w := "auto", "", 0
	applyConfig(cfg, map[string]bool{}, &toc, &img, &th, &w)
	if !toc || img != "sixel" || th != "/from/config.toml" || w != 70 {
		t.Fatalf("config not applied: toc=%v img=%q theme=%q width=%d", toc, img, th, w)
	}

	// Now the same config with every flag given explicitly.
	toc, img, th, w = false, "kitty", "/cli.toml", 120
	applyConfig(cfg, map[string]bool{"toc": true, "images": true, "theme": true, "w": true}, &toc, &img, &th, &w)
	if toc || img != "kitty" || th != "/cli.toml" || w != 120 {
		t.Errorf("command line was overridden: toc=%v img=%q theme=%q width=%d", toc, img, th, w)
	}
}

func TestConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-test")
	if got := configPath(""); got != "/tmp/xdg-test/mdv/config.json" {
		t.Errorf("default path = %q", got)
	}
	if got := configPath("/tmp/other.json"); got != "/tmp/other.json" {
		t.Errorf("explicit path = %q", got)
	}
	home, _ := os.UserHomeDir()
	if got := configPath("~/cfg.json"); got != filepath.Join(home, "cfg.json") {
		t.Errorf("tilde expansion = %q", got)
	}
}
