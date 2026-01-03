//go:build darwin
// +build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Output      OutputConfig      `json:"output"`
	Parse       ParseConfig       `json:"parse"`
	Show        ShowConfig        `json:"show"`
	AppleScript AppleScriptConfig `json:"applescript"`
}

type OutputConfig struct {
	Open    *bool `json:"open"`
	Print   *bool `json:"print"`
	Copy    *bool `json:"copy"`
	JSON    *bool `json:"json"`
	Plain   *bool `json:"plain"`
	DryRun  *bool `json:"dry_run"`
	Verbose *bool `json:"verbose"`
}

type ParseConfig struct {
	Calendar string `json:"calendar"`
	Note     string `json:"note"`
	Add      *bool  `json:"add"`
}

type ShowConfig struct {
	// Reserved for future defaults.
}

type AppleScriptConfig struct {
	Add   *bool `json:"add"`
	Run   *bool `json:"run"`
	Print *bool `json:"print"`
}

func loadConfigWithPath(path string) (*Config, error) {
	cfg := &Config{}

	userPath, projectPath := configPaths(path)
	if userPath != "" {
		readCfg, err := readConfigFile(userPath)
		if err != nil {
			return nil, err
		}
		mergeConfig(cfg, readCfg)
	}
	if projectPath != "" {
		readCfg, err := readConfigFile(projectPath)
		if err != nil {
			return nil, err
		}
		mergeConfig(cfg, readCfg)
	}

	applyEnvOverrides(cfg)

	return cfg, nil
}

func configPaths(override string) (string, string) {
	envPath := strings.TrimSpace(os.Getenv("FANTASTICAL_CONFIG"))
	userPath := envPath
	if userPath == "" {
		userPath = defaultUserConfigPath()
	}
	if strings.TrimSpace(override) != "" {
		userPath = strings.TrimSpace(override)
	}

	projectPath := ".fantastical.json"
	return userPath, projectPath
}

func defaultUserConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || strings.TrimSpace(dir) == "" {
		return ""
	}
	return filepath.Join(dir, "fantastical", "config.json")
}

func readConfigFile(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	return &cfg, nil
}

func mergeConfig(dst, src *Config) {
	if src == nil || dst == nil {
		return
	}

	if src.Output.Open != nil {
		dst.Output.Open = src.Output.Open
	}
	if src.Output.Print != nil {
		dst.Output.Print = src.Output.Print
	}
	if src.Output.Copy != nil {
		dst.Output.Copy = src.Output.Copy
	}
	if src.Output.JSON != nil {
		dst.Output.JSON = src.Output.JSON
	}
	if src.Output.Plain != nil {
		dst.Output.Plain = src.Output.Plain
	}
	if src.Output.DryRun != nil {
		dst.Output.DryRun = src.Output.DryRun
	}
	if src.Output.Verbose != nil {
		dst.Output.Verbose = src.Output.Verbose
	}

	if strings.TrimSpace(src.Parse.Calendar) != "" {
		dst.Parse.Calendar = src.Parse.Calendar
	}
	if strings.TrimSpace(src.Parse.Note) != "" {
		dst.Parse.Note = src.Parse.Note
	}
	if src.Parse.Add != nil {
		dst.Parse.Add = src.Parse.Add
	}

	if src.AppleScript.Add != nil {
		dst.AppleScript.Add = src.AppleScript.Add
	}
	if src.AppleScript.Run != nil {
		dst.AppleScript.Run = src.AppleScript.Run
	}
	if src.AppleScript.Print != nil {
		dst.AppleScript.Print = src.AppleScript.Print
	}
}

func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if v, ok := envBool("FANTASTICAL_DEFAULT_OPEN"); ok {
		cfg.Output.Open = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_DEFAULT_PRINT"); ok {
		cfg.Output.Print = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_DEFAULT_COPY"); ok {
		cfg.Output.Copy = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_DEFAULT_JSON"); ok {
		cfg.Output.JSON = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_DEFAULT_PLAIN"); ok {
		cfg.Output.Plain = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_DRY_RUN"); ok {
		cfg.Output.DryRun = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_VERBOSE"); ok {
		cfg.Output.Verbose = boolPtr(v)
	}

	if v, ok := envString("FANTASTICAL_DEFAULT_CALENDAR"); ok {
		cfg.Parse.Calendar = v
	}
	if v, ok := envString("FANTASTICAL_DEFAULT_NOTE"); ok {
		cfg.Parse.Note = v
	}
	if v, ok := envBool("FANTASTICAL_DEFAULT_ADD"); ok {
		cfg.Parse.Add = boolPtr(v)
	}

	if v, ok := envBool("FANTASTICAL_APPLESCRIPT_ADD"); ok {
		cfg.AppleScript.Add = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_APPLESCRIPT_RUN"); ok {
		cfg.AppleScript.Run = boolPtr(v)
	}
	if v, ok := envBool("FANTASTICAL_APPLESCRIPT_PRINT"); ok {
		cfg.AppleScript.Print = boolPtr(v)
	}
}

func envString(key string) (string, bool) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return "", false
	}
	return val, true
}

func envBool(key string) (bool, bool) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return false, false
	}
	return parsed, true
}

func boolPtr(v bool) *bool {
	return &v
}
