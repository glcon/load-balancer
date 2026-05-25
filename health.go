package main

import (
	"context"
	"log"
	"math/rand/v2"
	"net/http"
	"time"
)

func (b *Backend) Ping() {
	// create a context, we want the request to timeout eventually
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	healthURL := b.URL.String() + "/health"

	// request with context
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		b.updateStatus(false)
		return
	}

	// create a local client -> doesn't compete with the balancer
	response, err := b.HealthClient.Do(request)
	if err != nil {
		b.updateStatus(false)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		b.updateStatus(false)
		return
	}

	b.updateStatus(true)
}

func (b *Backend) updateStatus(isAlive bool) {
	// Set isAlive
	if b.Alive.CompareAndSwap(!isAlive, isAlive) {
		log.Printf("Backend %v status changed to %v", b.URL.String(), isAlive)
	}

	// Set the breaker back to closed if its open
	if isAlive && b.CircuitState.CompareAndSwap(stateOpen, stateClosed) {
		b.ConsecutiveFails.Store(0)
		log.Printf("Circuit breaker switched to CLOSED for backend %v", b.ID)
	}
}

func healthLoop(ctx context.Context, backend *Backend, interval time.Duration) {
	// avoid synced startup storms, delay each check by random 1-5 seconds
	delay := time.Duration(rand.IntN(3000)) * time.Millisecond
	time.Sleep(delay)

	// ticker sends a "go ahead" every INTERVAL seconds
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// select statement: checks if any of the cases can be run, if so, runs it
		// used for waiting on multiple chan operations at the same time
		select {
		// ctx.Done() will become ready once cancel() is called
		case <-ctx.Done():
			return
		case <-ticker.C:
			backend.Ping()
		}
	}
}
