package registry

import "testing"

func TestMustRegisterDuplicatePanics(t *testing.T) {
	r := New[string]()
	r.MustRegister("a", "first")
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration, got none")
		}
	}()
	r.MustRegister("a", "second")
}

func TestRegistryGetAndAll(t *testing.T) {
	r := New[int]()
	r.MustRegister("x", 1)
	r.MustRegister("y", 2)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}
	if all[0] != 1 || all[1] != 2 {
		t.Fatalf("expected registration order [1,2], got %v", all)
	}

	v, ok := r.Get("x")
	if !ok || *v != 1 {
		t.Fatalf("expected x=1, got %v ok=%v", v, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Fatal("expected missing key to be absent")
	}
}
