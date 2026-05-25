package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
)

type BackendConfig struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// stuff straight from json file
type MasterConfig struct {
	ListenAddr          string          `json:"listen_addr"`
	HealthCheckInterval int             `json:"health_check_interval"`
	Backends            []BackendConfig `json:"backends"`
}

func loadConfig(configPath string) (*MasterConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg MasterConfig
	err = json.Unmarshal(data, &cfg)
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
				log.Println("SIGHUP received. Reloading config.json.")

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
