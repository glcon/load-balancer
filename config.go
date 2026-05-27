package main

import (
	"context"
	"log"
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
	HealthCheckInterval int             `yaml:"health_check_interval"`
	Backends            []BackendConfig `yaml:"backends"`
}

func loadConfig(configPath string) (*MasterConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg MasterConfig
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Checks for SIGHUP (signal to hot swap)
func startSignalHandler(ctx context.Context, reg *BackendResigtry, configPath string) {
	// chan for signal to reach goroutine
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGHUP)

	go func() {
		log.Printf("Signal handler initialized. Send SIGHUP to reload config.")

		for {
			select {
			case <-ctx.Done():
				signal.Stop(sigChan)
				return

			case <-sigChan:
				log.Println("SIGHUP received. Reloading config.")

				masterConfig, err := loadConfig(configPath)
				if err != nil {
					log.Println("Hot reload FAILED: Could not load config. Previous state preserved.")
					continue
				}

				reg.Update(masterConfig.Backends)
				log.Println("Hot reload SUCCESS: registry updated.")
			}
		}
	}()
}
