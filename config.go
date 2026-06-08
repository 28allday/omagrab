package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds user preferences, persisted as JSON under ~/.config/omagrab.
type Config struct {
	AudioDir     string `json:"audio_dir"`
	VideoDir     string `json:"video_dir"`
	AudioFormat  string `json:"audio_format"`  // mp3 | opus | flac | m4a
	VideoQuality string `json:"video_quality"` // best | 2160 | 1440 | 1080 | 720
	Subtitles    bool   `json:"subtitles"`
}

// AudioFormats / VideoQualities are the values cycled through in the config view.
var (
	AudioFormats  = []string{"opus", "mp3", "m4a", "flac"}
	VideoQualities = []string{"best", "2160", "1440", "1080", "720"}
)

func defaultConfig() Config {
	return Config{
		AudioDir:     "~/Music",
		VideoDir:     "~/Videos",
		AudioFormat:  "opus",
		VideoQuality: "best",
		Subtitles:    false,
	}
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "omagrab", "config.json"), nil
}

// loadConfig reads the config file, writing defaults on first run.
func loadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := defaultConfig()
		return cfg, saveConfig(cfg)
	}
	if err != nil {
		return Config{}, err
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func saveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ExpandHome resolves a leading ~ to the user's home directory.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}
