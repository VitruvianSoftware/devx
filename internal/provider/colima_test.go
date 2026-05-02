package provider

import "testing"

func TestColimaProvider_Name(t *testing.T) {
	p := &ColimaProvider{}
	if p.Name() != "colima" {
		t.Errorf("expected colima, got %s", p.Name())
	}
}

func TestColimaProvider_ImplementsVMBackend(t *testing.T) {
	// Compile-time check
	var _ VMProvider = (*ColimaProvider)(nil)
}
