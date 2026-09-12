package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoutingDefaultsAndExplicitOptOutPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("port: 8317\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Routing.Strategy != "soonest-reset" || !cfg.Routing.SessionAffinity {
		t.Fatalf("defaults = %+v", cfg.Routing)
	}
	cfg.Routing.SessionAffinity = false
	cfg.Routing.Strategy = "soonest-reset"
	if err := SaveConfigPreserveComments(path, cfg); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Routing.SessionAffinity || reloaded.Routing.Strategy != "soonest-reset" {
		t.Fatalf("opt out lost = %+v", reloaded.Routing)
	}
}
