package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origConfigDir := configDirName
	// Override configDir by setting XDG_CONFIG_HOME (macOS/Linux)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	_ = origConfigDir

	// Manually test save/load using direct file operations
	dir := filepath.Join(tmpDir, configDirName)
	path := filepath.Join(dir, configFileName)

	cfg := &Config{
		ServerURL: "https://example.com",
		Token:     "test-token-abc123",
	}

	// Save
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := saveConfigToPath(path, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load
	loaded, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.ServerURL != cfg.ServerURL {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, cfg.ServerURL)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("Token = %q, want %q", loaded.Token, cfg.Token)
	}
}

func TestLoadConfigNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "config.json")
	cfg, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerURL != "" {
		t.Errorf("ServerURL = %q, want empty", cfg.ServerURL)
	}
	if cfg.Token != "" {
		t.Errorf("Token = %q, want empty", cfg.Token)
	}
}

func TestSaveConfigCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep")
	path := filepath.Join(dir, configFileName)

	cfg := &Config{ServerURL: "https://test.com"}
	if err := saveConfigToPath(path, cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if loaded.ServerURL != "https://test.com" {
		t.Errorf("ServerURL = %q, want %q", loaded.ServerURL, "https://test.com")
	}
}

func TestSaveConfigOverwritesToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), configFileName)

	// Save with initial token
	cfg := &Config{ServerURL: "https://example.com", Token: "old-token"}
	if err := saveConfigToPath(path, cfg); err != nil {
		t.Fatal(err)
	}

	// Update token
	cfg.Token = "new-token"
	if err := saveConfigToPath(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Token != "new-token" {
		t.Errorf("Token = %q, want %q", loaded.Token, "new-token")
	}
}

func TestConfigFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), configFileName)

	cfg := &Config{ServerURL: "https://example.com", Token: "secret"}
	if err := saveConfigToPath(path, cfg); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 600", perm)
	}
}
