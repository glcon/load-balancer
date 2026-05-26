package main

import (
	"errors"
	"log"
	"net/http"
	"time"
)

type Transport struct {
	LB *P2CLB
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	canRetry := req.Method == http.MethodGet || req.Method == http.MethodHead
	maxAttempts := 3

	if !canRetry {
		maxAttempts = 1
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt < maxAttempts; attempt++ {
		backend := t.LB.SelectBackend()
		if backend == nil {
			return nil, errors.New("503: no backends available")
		}

		// dynamic routing
		req.URL.Scheme = backend.URL.Scheme
		req.URL.Host = backend.URL.Host
		req.Host = backend.URL.Host

		if attempt > 0 && req.Body != nil && req.GetBody != nil {
			newBody, err := req.GetBody()
			if err != nil {
				return nil, err
			}

			req.Body = newBody
		}

		// run go standard roundtrip
		start := time.Now()

		backend.ActiveConnections.Add(1)
		resp, err := backend.Transport.RoundTrip(req)
		backend.ActiveConnections.Add(-1)

		latency := time.Since(start).Milliseconds() // returns an int64

		if err != nil || isSoftFailure(resp) {
			if resp != nil {
				resp.Body.Close()
			}

			fails := backend.ConsecutiveFails.Add(1)

			if fails > 3 {
				if backend.CircuitState.CompareAndSwap(stateClosed, stateOpen) {
					log.Printf("Circuit tripped to open for %v", backend.ID)
				}
			}

			lastErr = err
			continue
		}

		// reaching here means success
		backend.ConsecutiveFails.Store(0)
		backend.UpdateEWMA(latency)
		backend.LastRequest.Store(time.Now().UnixMilli())
		backend.TotalRequests.Add(1)

		return resp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

func isSoftFailure(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusGatewayTimeout
}
