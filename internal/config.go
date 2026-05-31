package internal

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"gopkg.in/yaml.v3"
)

type BackendConfig struct {
	ID  string `yaml:"id"`
	URL string `yaml:"url"`
}

// stuff straight from yml file
type MasterConfig struct {
	ListenAddr          string          `yaml:"listen_addr"`
	MetricsAddr         string          `yaml:"metrics_addr"`
	PprofAddr           string          `yaml:"pprof_addr"`
	HealthCheckInterval int             `yaml:"health_check_interval"`
	Backends            []BackendConfig `yaml:"backends"`
}

func LoadConfig(configPath string) (*MasterConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := MasterConfig{
		ListenAddr:  ":8080",
		MetricsAddr: ":9090",
		PprofAddr:   ":6060",
	}
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Checks for SIGHUP (signal to hot swap)
func StartSignalHandler(ctx context.Context, reg *BackendRegistry, configPath string) {
	// chan for signal to reach goroutine
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGHUP)

	go func() {
		slog.Info("Signal handler initialized", "configPath", configPath)

		for {
			select {
			case <-ctx.Done():
				signal.Stop(sigChan)
				return

			case <-sigChan:
				slog.Info("SIGHUP received", "configPath", configPath)

				masterConfig, err := LoadConfig(configPath)
				if err != nil {
					slog.Error("Hot reload failed: could not load config", "configPath", configPath, "error", err)
					continue
				}

				reg.Update(masterConfig.Backends)
				slog.Info("Hot reload success: registry updated", "configPath", configPath)
			}
		}
	}()
}
