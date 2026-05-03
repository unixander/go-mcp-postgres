package test

import (
	"testing"

	"mcp-postgres/internal/config"
	"mcp-postgres/internal/resolver"
)

func makeResolver() *resolver.Resolver {
	cfg := &config.Config{
		Admin:            config.Admin{Username: "u", Password: "p", Port: 5432},
		HostnameTemplate: "{group}-{contour}.example.com",
		Contours:         []string{"lkoz", "lkoo"},
		Groups: []config.GroupConfig{
			{Name: "alpha"},
			{
				Name:      "mars",
				Overrides: map[string]string{"lkoo": "mars-special.example.com"},
			},
		},
		Defaults: config.Defaults{ConnectTimeoutS: 5, DefaultDatabase: "postgres"},
	}
	return resolver.New(cfg)
}

func TestResolveHostTemplate(t *testing.T) {
	res := makeResolver()
	host, err := res.ResolveHost("alpha", "lkoz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "alpha-lkoz.example.com"
	if host != want {
		t.Errorf("got %q, want %q", host, want)
	}
}

func TestResolveHostOverride(t *testing.T) {
	res := makeResolver()
	host, err := res.ResolveHost("mars", "lkoo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "mars-special.example.com"
	if host != want {
		t.Errorf("got %q, want %q", host, want)
	}
}

func TestResolveHostOverrideFallsBackToTemplate(t *testing.T) {
	res := makeResolver()
	host, err := res.ResolveHost("mars", "lkoz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "mars-lkoz.example.com"
	if host != want {
		t.Errorf("got %q, want %q", host, want)
	}
}

func TestResolveHostUnknownGroup(t *testing.T) {
	res := makeResolver()
	_, err := res.ResolveHost("nonexistent", "lkoz")
	if err == nil {
		t.Fatal("expected error for unknown group")
	}
}

func TestResolveDSN(t *testing.T) {
	res := makeResolver()
	dsn, err := res.ResolveDSN("alpha", "lkoz", "mydb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "postgres://u:p@alpha-lkoz.example.com:5432/mydb?connect_timeout=5"
	if dsn != want {
		t.Errorf("got %q, want %q", dsn, want)
	}
}
