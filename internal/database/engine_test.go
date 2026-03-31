package database

import "testing"

func TestConnStringPostgres(t *testing.T) {
	e := Registry["postgres"]
	got := e.ConnString(5432)
	expected := "postgresql://devx:devx@localhost:5432/devx"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConnStringRedis(t *testing.T) {
	e := Registry["redis"]
	got := e.ConnString(6379)
	expected := "redis://localhost:6379"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConnStringMySQL(t *testing.T) {
	e := Registry["mysql"]
	got := e.ConnString(3306)
	expected := "mysql://devx:devx@localhost:3306/devx"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConnStringMongo(t *testing.T) {
	e := Registry["mongo"]
	got := e.ConnString(27017)
	expected := "mongodb://devx:devx@localhost:27017"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestSupportedEngines(t *testing.T) {
	engines := SupportedEngines()
	if len(engines) != 4 {
		t.Errorf("expected 4 engines, got %d", len(engines))
	}
}

func TestRegistryHasAllEngines(t *testing.T) {
	for _, name := range SupportedEngines() {
		if _, ok := Registry[name]; !ok {
			t.Errorf("engine %q not found in registry", name)
		}
	}
}
