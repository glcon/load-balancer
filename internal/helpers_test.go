package internal

import (
	"net/http"
	"net/url"
	"testing"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}

	return parsed
}

func testBackend(t *testing.T, id, rawURL string, alive bool, circuitState int32, ewma int64) *Backend {
	t.Helper()

	backend := &Backend{
		ID:               id,
		URL:              mustParseURL(t, rawURL),
		Transport:        &http.Transport{},
		HealthLoopCancel: func() {},
	}
	backend.Alive.Store(alive)
	backend.CircuitState.Store(circuitState)
	backend.EWMA.Store(ewma)

	return backend
}
