package internal

import (
	"net/http"
	"testing"
)

func TestUpdateEWMA(t *testing.T) {
	t.Parallel()

	backend := &Backend{}

	backend.UpdateEWMA(100)
	if got := backend.EWMA.Load(); got != 100 {
		t.Fatalf("EWMA after first update = %d, want %d", got, 100)
	}

	backend.UpdateEWMA(200)
	if got := backend.EWMA.Load(); got != 120 {
		t.Fatalf("EWMA after second update = %d, want %d", got, 120)
	}
}

func TestIsSoftFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code int
		want bool
	}{
		{name: "bad gateway", code: http.StatusBadGateway, want: true},
		{name: "service unavailable", code: http.StatusServiceUnavailable, want: true},
		{name: "gateway timeout", code: http.StatusGatewayTimeout, want: true},
		{name: "success", code: http.StatusOK, want: false},
		{name: "client error", code: http.StatusBadRequest, want: false},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{StatusCode: testCase.code}
			if got := isSoftFailure(resp); got != testCase.want {
				t.Fatalf("isSoftFailure(%d) = %v, want %v", testCase.code, got, testCase.want)
			}
		})
	}
}

func TestBackendAvailability(t *testing.T) {
	t.Parallel()

	backend := testBackend(t, "healthy", "http://127.0.0.1:9001", true, stateClosed, 10)
	if !backend.IsAvailable() {
		t.Fatal("expected backend to be available when alive and circuit is closed")
	}

	backend.Alive.Store(false)
	if backend.IsAvailable() {
		t.Fatal("expected backend to be unavailable after Alive is cleared")
	}

	backend.Alive.Store(true)
	backend.CircuitState.Store(stateOpen)
	if backend.IsAvailable() {
		t.Fatal("expected backend to be unavailable when circuit is open")
	}
}
