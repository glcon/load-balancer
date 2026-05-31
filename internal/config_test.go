package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yml")
	config := []byte(`listen_addr: ":8080"
health_check_interval: 7
backends:
  - id: "api-1"
    url: "http://127.0.0.1:9001"
  - id: "api-2"
    url: "http://127.0.0.1:9002"
`)
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if loaded.ListenAddr != ":8080" {
		t.Fatalf("ListenAddr = %q, want %q", loaded.ListenAddr, ":8080")
	}
	if loaded.MetricsAddr != ":9090" {
		t.Fatalf("MetricsAddr = %q, want %q", loaded.MetricsAddr, ":9090")
	}
	if loaded.PprofAddr != ":6060" {
		t.Fatalf("PprofAddr = %q, want %q", loaded.PprofAddr, ":6060")
	}
	if loaded.HealthCheckInterval != 7 {
		t.Fatalf("HealthCheckInterval = %d, want %d", loaded.HealthCheckInterval, 7)
	}
	if len(loaded.Backends) != 2 {
		t.Fatalf("len(Backends) = %d, want %d", len(loaded.Backends), 2)
	}
	if loaded.Backends[0].ID != "api-1" || loaded.Backends[1].URL != "http://127.0.0.1:9002" {
		t.Fatalf("loaded backends = %#v", loaded.Backends)
	}
}
