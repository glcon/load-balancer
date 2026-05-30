package internal

import (
	"fmt"
	"log/slog"
	"sync/atomic"
)

type BackendRegistry struct {
	value               atomic.Value // Always of type Snapshot
	healthCheckInterval int
}

type Snapshot struct {
	IDMap       map[string]*Backend
	PointerList []*Backend
}

func MakeRegistry(masterConfig *MasterConfig) (*BackendRegistry, error) {
	if masterConfig.HealthCheckInterval < 1 {
		return nil, fmt.Errorf("health_check_interval must be at least 1 second")
	}

	registryMap := make(map[string]*Backend)
	registryList := make([]*Backend, 0)

	for _, cfg := range masterConfig.Backends {
		b, err := NewBackend(cfg, masterConfig.HealthCheckInterval)
		if err != nil {
			return nil, err
		}

		registryMap[b.ID] = b
		registryList = append(registryList, b)
	}

	reg := &BackendRegistry{}
	reg.healthCheckInterval = masterConfig.HealthCheckInterval

	s := &Snapshot{
		IDMap:       registryMap,
		PointerList: registryList,
	}

	reg.value.Store(s)

	return reg, nil
}

func (reg *BackendRegistry) Update(newConfigs []BackendConfig) {
	snapshot := reg.value.Load().(*Snapshot)
	current := snapshot.IDMap

	// copy that will be returned once finished
	next := make(map[string]*Backend, len(current))
	for id, backend := range current {
		next[id] = backend
	}

	reg.addNewBackends(next, current, newConfigs)

	// Deletes orphans from reconciled list and returns a list of them
	orphans := identifyOrphans(next, current, newConfigs)

	// build standard list to store in snapshot
	pointerList := make([]*Backend, 0, len(next))
	for _, backend := range next {
		pointerList = append(pointerList, backend)
	}

	updatedSnapshot := &Snapshot{
		IDMap:       next,
		PointerList: pointerList,
	}

	// hot swap
	reg.value.Store(updatedSnapshot)

	// start shutting down AFTER the new registry is live
	for _, orphan := range orphans {
		orphan.Alive.Store(false)
		go orphan.Drain()
	}
}

func identifyOrphans(reconciled map[string]*Backend, currentSet map[string]*Backend, newConfigs []BackendConfig) []*Backend {
	// build a hash map for O(1) lookup
	newConfigsHash := make(map[string]struct{})
	for _, cfg := range newConfigs {
		newConfigsHash[cfg.ID] = struct{}{}
	}

	var orphans []*Backend
	for id, backend := range currentSet {
		_, ok := newConfigsHash[id]
		if ok == false {
			orphans = append(orphans, backend)
			delete(reconciled, id)
		}
	}

	return orphans
}

func (reg *BackendRegistry) addNewBackends(reconciled map[string]*Backend, currentSet map[string]*Backend, newConfigs []BackendConfig) {
	for _, cfg := range newConfigs {
		_, ok := currentSet[cfg.ID]

		if ok == false {
			newBackend, err := NewBackend(cfg, reg.healthCheckInterval)

			if err != nil {
				slog.Error("Could not create backend", "backend", cfg.ID, "error", err)
				continue
			}

			reconciled[cfg.ID] = newBackend
		}
	}
}
