package main

import (
	"math/rand/v2"
)

type P2CLB struct {
	Registry *BackendResigtry
}

func (p *P2CLB) SelectBackend() *Backend {
	snapShot := p.Registry.value.Load().(*Snapshot)
	backendList := snapShot.PointerList

	// 0 or 1 backends
	listLen := len(backendList)
	if listLen == 0 {
		return nil
	}
	if listLen == 1 {
		b := backendList[0]

		if b.IsAvailable() {
			return b
		}
		return nil
	}

	// three attempts to find a healthy balancer
	for attempt := 0; attempt < 3; attempt++ {
		index1 := rand.IntN(listLen)
		index2 := rand.IntN(listLen)
		if index1 == index2 {
			index2 = (index2 + 1) % listLen
		}

		b1 := backendList[index1]
		b2 := backendList[index2]

		if b1.IsAvailable() && b2.IsAvailable() {
			if b1.EWMA.Load() < b2.EWMA.Load() {
				return b1
			}
			return b2
		}

		if b1.IsAvailable() {
			return b1
		}
		if b2.IsAvailable() {
			return b2
		}
	}

	return nil
}

func (b *Backend) IsAvailable() bool {
	if b.Alive.Load() && b.CircuitState.Load() == stateClosed {
		return true
	}
	return false
}
