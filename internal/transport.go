package internal

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
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

		// Shallow-copy the request struct and the URL struct so we can set the
		// target backend without mutating the caller's request (required by the
		// http.RoundTripper contract). This avoids the full header-map allocation
		// that req.Clone() performs on every request.
		urlCopy := *req.URL
		urlCopy.Scheme = backend.URL.Scheme
		urlCopy.Host = backend.URL.Host

		attemptReq := *req
		attemptReq.URL = &urlCopy
		attemptReq.Host = backend.URL.Host

		if attempt > 0 && req.Body != nil && req.GetBody != nil {
			newBody, err := req.GetBody()
			if err != nil {
				return nil, err
			}

			attemptReq.Body = newBody
		}

		start := time.Now()

		var resp *http.Response
		var err error

		func() {
			// log connections for balancer and prometheus
			backend.ActiveConnections.Add(1)
			defer backend.ActiveConnections.Add(-1)

			MetricsActiveConnections.WithLabelValues(backend.ID).Inc()
			defer MetricsActiveConnections.WithLabelValues(backend.ID).Dec()

			resp, err = backend.Transport.RoundTrip(&attemptReq)
		}()

		duration := time.Since(start)
		latency := duration.Seconds()

		// latency histogram
		MetricsRequestDuration.WithLabelValues(backend.ID).Observe(latency)

		if err != nil || isSoftFailure(resp) {
			if resp != nil {
				resp.Body.Close()
				// track soft failure errors
				MetricsRequestsTotal.WithLabelValues(backend.ID, strconv.Itoa(resp.StatusCode)).Inc()
			} else {
				// track network failures (timeouts, refused conns)
				MetricsRequestsTotal.WithLabelValues(backend.ID, "network_error").Inc()
			}

			fails := backend.ConsecutiveFails.Add(1)

			if fails > 3 {
				if backend.CircuitState.CompareAndSwap(stateClosed, stateOpen) {
					slog.Warn("Circuit tripped to open", "backend", backend.ID)
				}
			}

			lastErr = err
			continue
		}

		MetricsRequestsTotal.WithLabelValues(backend.ID, strconv.Itoa(resp.StatusCode)).Inc()

		// reaching here means success
		backend.ConsecutiveFails.Store(0)
		backend.UpdateEWMA(duration.Milliseconds())
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
