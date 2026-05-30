package internal

import "testing"

func TestSelectBackend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		backends []*Backend
		wantID   string
		wantNil  bool
	}{
		{name: "empty snapshot", backends: nil, wantNil: true},
		{name: "single healthy backend", backends: []*Backend{testBackend(t, "healthy", "http://127.0.0.1:9001", true, stateClosed, 10)}, wantID: "healthy"},
		{name: "single unavailable backend", backends: []*Backend{testBackend(t, "down", "http://127.0.0.1:9002", false, stateClosed, 5)}, wantNil: true},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			registry := &BackendRegistry{}
			idMap := make(map[string]*Backend, len(testCase.backends))
			for _, backend := range testCase.backends {
				idMap[backend.ID] = backend
			}
			registry.value.Store(&Snapshot{IDMap: idMap, PointerList: testCase.backends})

			selected := (&P2CLB{Registry: registry}).SelectBackend()
			if testCase.wantNil {
				if selected != nil {
					t.Fatalf("SelectBackend() = %#v, want nil", selected)
				}
				return
			}

			if selected == nil {
				t.Fatal("SelectBackend() = nil, want backend")
			}
			if selected.ID != testCase.wantID {
				t.Fatalf("SelectBackend() = %q, want %q", selected.ID, testCase.wantID)
			}
		})
	}
}
