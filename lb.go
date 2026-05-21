package main

import (
	"math/rand/v2"
	"sync/atomic"
)

type LoadBalancer interface {
	Next() *Backend
}

type RoundRobinLB struct {
	backends []*Backend
	counter  int64
}

func NewRoundRobinLB(bs []*Backend) *RoundRobinLB {
	return &RoundRobinLB{backends: bs}
}

func (rr *RoundRobinLB) Next() *Backend {
	if len(rr.backends) == 0 {
		return nil
	}

	// starts at second backend, atomic to prevent contention
	index := atomic.AddInt64(&rr.counter, 1) % int64(len(rr.backends))

	return rr.backends[index]
}

type P2CLB struct {
	backends []*Backend
	rand     *rand.Rand
}

func NewP2CLB(bs []*Backend) *P2CLB {
	return &P2CLB{
		backends: bs,
	}
}

func (p *P2CLB) Next() *Backend {
	numBackends := len(p.backends)

	if numBackends == 0 {
		return nil
	}
	if numBackends == 1 {
		return p.backends[0]
	}

	// pick two random ones, cannot be equal
	// math/rand/v2 is by default thread-safe
	index1 := rand.IntN(numBackends)
	index2 := rand.IntN(numBackends)
	for index1 == index2 {
		index2 = rand.IntN(numBackends)
	}

	backend1 := p.backends[index1]
	backend2 := p.backends[index2]

	if atomic.LoadInt64(&backend1.ActiveConnections) >=
		atomic.LoadInt64(&backend2.ActiveConnections) {
		return backend2
	} else {
		return backend1
	}

}
