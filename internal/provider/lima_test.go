package provider

import "testing"

func TestLimaProvider_Name(t *testing.T) {
	p := &LimaProvider{}
	if p.Name() != "lima" {
		t.Errorf("expected lima, got %s", p.Name())
	}
}

func TestLimaProvider_ImplementsVMBackend(t *testing.T) {
	// Compile-time check
	var _ VMProvider = (*LimaProvider)(nil)
}
