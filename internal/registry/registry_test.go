package registry

import "testing"

func TestMustRegisterDuplicatePanics(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestRegistryUpdateMutatesInPlace(t *testing.T) {
	t.Parallel()
	r := New[int]()
	r.MustRegister("a", 1)
	r.MustRegister("b", 2)

	r.Update("a", 100)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries after Update, got %d", len(all))
	}
	if all[0] != 100 {
		t.Errorf("Update did not change order-position 0: got %d, want 100", all[0])
	}
	if all[1] != 2 {
		t.Errorf("Update of %q affected unrelated entry %q: got %d, want 2", "a", "b", all[1])
	}
	v, ok := r.Get("a")
	if !ok || *v != 100 {
		t.Fatalf("Get(a) after Update = %v, ok=%v; want 100, true", v, ok)
	}
}

func TestRegistryUpdateUnknownNamePanics(t *testing.T) {
	t.Parallel()
	r := New[int]()
	r.MustRegister("a", 1)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on Update of unregistered name, got none")
		}
	}()
	r.Update("missing", 2)
}
