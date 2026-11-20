package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Only user-approved non-secret paths are persisted. Never add key paths,
// passphrase paths, remote credential config or credential values here.
type rememberedPaths struct {
	Version     int    `json:"version"`
	Executable  string `json:"executable"`
	Backup      string `json:"local_backup_folder"`
	Destination string `json:"last_destination"`
}

func settingsFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "VaultOfflineRecovery", "paths.json"), nil
}

func loadRememberedPaths(path string) (rememberedPaths, error) {
	var v rememberedPaths
	raw, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	if len(raw) > 8192 {
		return v, errors.New("settings file is too large")
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, err
	}
	if v.Version != 1 {
		return v, errors.New("unrecognized settings version")
	}
	return v, nil
}

func saveRememberedPaths(path string, v rememberedPaths) error {
	v.Version = 1
	v.Executable = strings.TrimSpace(v.Executable)
	v.Backup = strings.TrimSpace(v.Backup)
	v.Destination = strings.TrimSpace(v.Destination)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Exclusive temp file + atomic rename; never contain key contents or
	// passphrases. Parent directory inherits the current user's ACL on Windows.
	f, err := os.CreateTemp(dir, "paths-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	// Windows does not let Rename replace an existing file. This settings
	// file is not sensitive but losing settings is preferable to changing
	// recovery behavior. Remove and rename on this user's own opt-in file.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(f.Name(), path)
}

func deleteRememberedPaths(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
