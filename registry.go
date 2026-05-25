package main

import (
	"log"
	"sync/atomic"
)

type BackendResigtry struct {
	value atomic.Value // Always of type Snapshot
}

type Snapshot struct {
	IDMap       map[string]*Backend
	PointerList []*Backend
}

func makeRegistry(backendConfigs []BackendConfig) (*BackendResigtry, error) {
	registryMap := make(map[string]*Backend)
	registryList := make([]*Backend, 0)

	for _, cfg := range backendConfigs {
		b, err := NewBackend(cfg)
		if err != nil {
			return nil, err
		}

		registryMap[b.ID] = b
		registryList = append(registryList, b)
	}

	reg := &BackendResigtry{}

	s := &Snapshot{
		IDMap:       registryMap,
		PointerList: registryList,
	}

	reg.value.Store(s)

	return reg, nil
}

func (reg *BackendResigtry) Update(newConfigs []BackendConfig) {
	snapShot := reg.value.Load().(*Snapshot)
	currentSet := snapShot.IDMap

	// copy that will be returned once finished
	reconciledMap := make(map[string]*Backend, len(currentSet))
	for k, v := range currentSet {
		reconciledMap[k] = v
	}

	// Edit reconciled list
	addNewBackends(reconciledMap, currentSet, newConfigs)
	deleteOldBackends(reconciledMap, currentSet, newConfigs)

	// build standard list to store in snapshot
	reconciledList := make([]*Backend, 0)
	for _, backend := range reconciledMap {
		reconciledList = append(reconciledList, backend)
	}

	s := &Snapshot{
		IDMap:       reconciledMap,
		PointerList: reconciledList,
	}

	// hot swap
	reg.value.Store(s)
}

func deleteOldBackends(reconciled map[string]*Backend, currentSet map[string]*Backend, newConfigs []BackendConfig) {
	// build a hash map for O(1) lookup
	newConfigsHash := make(map[string]struct{})
	for _, cfg := range newConfigs {
		newConfigsHash[cfg.ID] = struct{}{}
	}

	for id, backend := range currentSet {
		_, ok := newConfigsHash[id]
		if ok == false {
			backend.Cancel() // logic should be expanded later to wind down
			delete(reconciled, id)
		}
	}
}

func addNewBackends(reconciled map[string]*Backend, currentSet map[string]*Backend, newConfigs []BackendConfig) {
	for _, cfg := range newConfigs {
		_, ok := currentSet[cfg.ID]

		if ok == false {
			newBackend, err := NewBackend(cfg)

			if err != nil {
				log.Printf("Could not create a backend for %v\n", cfg.ID)
				continue
			}

			reconciled[cfg.ID] = newBackend
		}
	}
}
