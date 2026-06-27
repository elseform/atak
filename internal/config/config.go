package config

import (
	"encoding/json"
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed configs/compression_profiles.json
var defaultProfilesJSON []byte

// Profile defines a texture compression target matched by filename pattern.
type Profile struct {
	Name         string   `json:"name"`
	Format       string   `json:"format"`
	Patterns     []string `json:"patterns"`
	Exclude      []string `json:"exclude,omitempty"`
	GenerateMips bool     `json:"generateMips,omitempty"`
}

type profileFile struct {
	ExcludePatterns  []string  `json:"excludePatterns,omitempty"`
	MinFileSizeBytes int       `json:"minFileSizeBytes,omitempty"`
	Profiles         []Profile `json:"profiles"`
}

// Config holds all user-persisted preferences.
type Config struct {
	ModsDir        string   `json:"modsDir"`
	BackupDir      string   `json:"backupDir"`
	WorkerCount    int      `json:"workerCount"`
	BackupLevel    int      `json:"backupLevel,omitempty"`
	ScanExclusions []string `json:"scanExclusions,omitempty"`
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "atak"), nil
}

// ConfigDir returns the atak config directory path.
func ConfigDir() (string, error) {
	return configDir()
}

// Load reads config.json from the user config dir, returning defaults if absent.
func Load() (*Config, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultConfig(), nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.ScanExclusions) == 0 {
		cfg.ScanExclusions = []string{".*", "downloads", "Downloads"}
	}
	if cfg.BackupLevel == 0 {
		cfg.BackupLevel = 6
	}
	return &cfg, nil
}

// Save writes cfg to config.json in the user config dir.
func Save(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}

// IsFirstRun returns true if no config.json exists yet.
func IsFirstRun() bool {
	dir, err := configDir()
	if err != nil {
		return true
	}
	_, err = os.Stat(filepath.Join(dir, "config.json"))
	return errors.Is(err, os.ErrNotExist)
}

// LoadProfiles returns compression profiles, global exclude patterns, minimum file
// size in bytes, and whether profiles.json was just created for the first time.
func LoadProfiles() ([]Profile, []string, int, bool, error) {
	dir, err := configDir()
	if err != nil {
		return nil, nil, 0, false, err
	}
	userPath := filepath.Join(dir, "profiles.json")
	data, err := os.ReadFile(userPath)
	created := false
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, 0, false, err
		}
		if err := os.WriteFile(userPath, defaultProfilesJSON, 0644); err != nil {
			return nil, nil, 0, false, err
		}
		data = defaultProfilesJSON
		created = true
	} else if err != nil {
		return nil, nil, 0, false, err
	}
	var pf profileFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return nil, nil, 0, false, err
	}
	if pf.MinFileSizeBytes == 0 {
		pf.MinFileSizeBytes = 1024
	}
	return pf.Profiles, pf.ExcludePatterns, pf.MinFileSizeBytes, created, nil
}

func defaultConfig() *Config {
	return &Config{
		ModsDir:        detectModsDir(),
		WorkerCount:    1,
		BackupLevel:    6,
		ScanExclusions: []string{".*", "downloads", "Downloads", "G.A.M.M.A. UI"},
	}
}

func detectModsDir() string {
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{
			`C:\Games\GAMMA\mods`,
			`D:\Games\GAMMA\mods`,
			`D:\GAMMA\mods`,
			`C:\GAMMA\mods`,
		}
		if env := os.Getenv("MO2_GAME_PATH"); env != "" {
			candidates = append([]string{filepath.Join(env, "mods")}, candidates...)
		}
	} else {
		home, _ := os.UserHomeDir()
		candidates = []string{
			filepath.Join(home, "Games", "GAMMA", "mods"),
			filepath.Join(home, "GAMMA", "mods"),
		}
		if env := os.Getenv("MO2_GAME_PATH"); env != "" {
			candidates = append([]string{filepath.Join(env, "mods")}, candidates...)
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
