package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

const (
	stateClosed = 0
	stateOpen   = 1
)

type Backend struct {
	ID        string
	URL       *url.URL
	Transport *http.Transport

	// managed by the active health checker
	Alive             atomic.Bool
	ActiveConnections atomic.Int64
	TotalRequests     atomic.Int64

	// takes on either stateClosed or stateOpen
	ConsecutiveFails atomic.Int64
	LastRequest      atomic.Int64
	EWMA             atomic.Int64

	// tripped when user requests fail
	CircuitState atomic.Int32

	// each backend gets a small client for health pings
	HealthClient     http.Client
	HealthLoopCancel context.CancelFunc
}

func NewBackend(cfg BackendConfig) (*Backend, error) {
	target, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	backendTransport := http.DefaultTransport.(*http.Transport).Clone()
	backendTransport.MaxIdleConns = 1000
	backendTransport.MaxIdleConnsPerHost = 100
	backendTransport.IdleConnTimeout = 90 * time.Second
	backendTransport.DisableCompression = true
	backendTransport.ForceAttemptHTTP2 = true

	ctx, cancel := context.WithCancel(context.Background())
	b := &Backend{
		ID:               cfg.ID,
		URL:              target,
		Transport:        backendTransport,
		HealthLoopCancel: cancel,
		HealthClient: http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}

	// Run the backend's personal health checking loop
	go healthLoop(ctx, b, 5*time.Second)

	return b, nil
}

func (b *Backend) UpdateEWMA(latency int64) {
	for {
		oldEWMA := b.EWMA.Load()
		var newEWMA int64

		if oldEWMA == 0 {
			newEWMA = latency
		} else {
			// expression expanded to avoid floating point ops
			// alpha is 0.2 here
			newEWMA = ((oldEWMA * 4) + latency) / 5
		}

		// don't even try cas if they are the same
		if oldEWMA == newEWMA {
			break
		}

		if b.EWMA.CompareAndSwap(oldEWMA, newEWMA) {
			break
		}
	}
}

func (b *Backend) Drain() {
	timeout := time.After(15 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if b.ActiveConnections.Load() == 0 {
			break
		}

		select {
		case <-timeout:
			log.Printf("Backend %v drain timeout. Forcing close.", b.ID)
			b.HealthLoopCancel()
			return
		case <-ticker.C:
		}
	}

	b.HealthLoopCancel()
	log.Printf("Backend %v drained successfully.", b.ID)
}
