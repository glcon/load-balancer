package internal

import "testing"

func TestRegistryUpdate(t *testing.T) {
	t.Parallel()

	backendA := testBackend(t, "api-1", "http://127.0.0.1:9001", true, stateClosed, 10)
	backendB := testBackend(t, "api-2", "http://127.0.0.1:9002", true, stateClosed, 20)

	registry := &BackendRegistry{}
	registry.value.Store(&Snapshot{
		IDMap: map[string]*Backend{
			backendA.ID: backendA,
			backendB.ID: backendB,
		},
		PointerList: []*Backend{backendA, backendB},
	})

	registry.Update([]BackendConfig{
		{ID: "api-1", URL: "http://127.0.0.1:9001"},
		{ID: "api-3", URL: "http://127.0.0.1:9003"},
	})

	snapshot := registry.value.Load().(*Snapshot)
	if len(snapshot.IDMap) != 2 {
		t.Fatalf("len(IDMap) = %d, want %d", len(snapshot.IDMap), 2)
	}
	if snapshot.IDMap["api-1"] != backendA {
		t.Fatal("existing backend pointer was not preserved")
	}
	if _, ok := snapshot.IDMap["api-2"]; ok {
		t.Fatal("removed backend still present after update")
	}
	if snapshot.IDMap["api-3"] == nil {
		t.Fatal("new backend was not added")
	}
	if backendC := snapshot.IDMap["api-3"]; backendC != nil {
		backendC.HealthLoopCancel()
	}
}
