package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

const (
	stateClosed = 0
	stateOpen   = 1
)

type Backend struct {
	ID                string
	URL               *url.URL
	Proxy             *httputil.ReverseProxy
	Alive             atomic.Bool
	ActiveConnections atomic.Int64
	TotalRequests     atomic.Int64

	// 0 is closed, 1 is open
	ConsecutiveFails atomic.Int64
	LastRequest      atomic.Int64
	EWMA             atomic.Int64
	CircuitState     atomic.Int32

	// each backend gets a small client for health pings
	HealthClient http.Client
	Cancel       context.CancelFunc
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func NewBackend(cfg BackendConfig) (*Backend, error) {
	target, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// modify director function
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Header.Set("X-Proxy-Source", "balancer")

		// So dest. server recognizes the request
		req.Host = target.Host
	}

	ctx, cancel := context.WithCancel(context.Background())
	b := &Backend{
		ID:     cfg.ID,
		URL:    target,
		Proxy:  proxy,
		Cancel: cancel,
		HealthClient: http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		},
	}

	proxy.ErrorHandler = customErrorHandler(b)

	proxy.ModifyResponse = func(res *http.Response) error {
		b.ConsecutiveFails.Store(0)

		// return nil to leave the response unmodified
		return nil
	}

	// Run the backend's personal health checking loop
	go healthLoop(ctx, b, 5*time.Second)

	return b, nil
}

func (b *Backend) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	b.ActiveConnections.Add(1)
	defer b.ActiveConnections.Add(-1)

	b.Proxy.ServeHTTP(w, r)

	latency := time.Since(start).Milliseconds()

	// ensure emwa swap is atomic and up to date using a CAS
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

	b.LastRequest.Store(time.Now().UnixMilli())
	b.TotalRequests.Add(1)
}

func customErrorHandler(b *Backend) func(w http.ResponseWriter, r *http.Request, e error) {
	return func(w http.ResponseWriter, r *http.Request, e error) {
		if errors.Is(e, context.Canceled) {
			log.Printf("Proxy note: Client cancelled request to backend %v. Ignoring.", b.ID)
			return
		}

		statusCode := http.StatusBadGateway
		errMessage := "Backend is unreachable"

		var netErr net.Error
		if errors.As(e, &netErr) && netErr.Timeout() {
			statusCode = http.StatusGatewayTimeout
			errMessage = "Backend server timed out responding to request."
		}

		// circuit breaking logic
		log.Printf("Proxy error on backend %s [%d]: %v", b.ID, statusCode, e)

		fails := b.ConsecutiveFails.Add(1)

		if fails >= 3 {
			if b.CircuitState.CompareAndSwap(stateClosed, stateOpen) {
				log.Printf("Circuit breaker tripped to OPEN for backend %v", b.ID)
			}
		}

		// set headers and send response
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Proxy-Error", "true")
		w.WriteHeader(statusCode)

		givenError := ErrorResponse{
			Error:   http.StatusText(statusCode),
			Message: errMessage,
		}

		_ = json.NewEncoder(w).Encode(givenError)
	}
}
