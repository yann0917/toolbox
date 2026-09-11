package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	setHome(t, t.TempDir())
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Server.Port)
	}
	if !strings.HasSuffix(cfg.DataDir, ".toolbox/data") {
		t.Errorf("DataDir = %q", cfg.DataDir)
	}
}

func TestSetAndLoadSecretMasked(t *testing.T) {
	setHome(t, t.TempDir())
	if err := Set("volc.speech.app_id", "123456789"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("perm = %v, want -rwx for 0600", info.Mode().Perm())
	}
	kvs, err := List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, kv := range kvs {
		if kv.Key == "volc.speech.app_id" {
			found = true
			if kv.Value != "1******9" && !strings.Contains(kv.Value, "*") {
				t.Errorf("secret not masked: %q", kv.Value)
			}
		}
	}
	if !found {
		t.Error("app_id not listed")
	}
	cfg, _ := Load()
	if cfg.Volc.Speech.AppID != "123456789" {
		t.Errorf("AppID = %q", cfg.Volc.Speech.AppID)
	}
	_ = filepath.Join // keep import
}

func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // windows
}
