package test

import (
	"os"
	"path/filepath"
	"testing"

	"mcp-postgres/internal/config"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(content)
	f.Close()
	return f.Name()
}

func TestConfigLoadBasic(t *testing.T) {
	t.Setenv("PG_ADMIN_USER", "admin")
	t.Setenv("PG_ADMIN_PASSWORD", "secret")

	path := writeYAML(t, `
admin:
  username: ${PG_ADMIN_USER}
  password: ${PG_ADMIN_PASSWORD}
  port: 5432
hostname_template: "{group}-{contour}.example.com"
contours: [lkoz, lkoo]
groups:
  - name: alpha
  - name: beta
    writable: true
defaults:
  statement_timeout_ms: 10000
  max_rows: 500
  pool_min_size: 0
  pool_max_size: 2
  connect_timeout_s: 3
  default_database: mydb
`)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Admin.Username != "admin" {
		t.Errorf("username: got %q", cfg.Admin.Username)
	}
	if cfg.Admin.Password != "secret" {
		t.Errorf("password: got %q", cfg.Admin.Password)
	}
	if len(cfg.Groups) != 2 {
		t.Errorf("groups: got %d", len(cfg.Groups))
	}
	if !cfg.Groups[1].Writable {
		t.Error("beta should be writable")
	}
}

func TestConfigMissingEnv(t *testing.T) {
	os.Unsetenv("PG_ADMIN_USER")
	os.Unsetenv("PG_ADMIN_PASSWORD")

	path := writeYAML(t, `
admin:
  username: ${PG_ADMIN_USER}
  password: ${PG_ADMIN_PASSWORD}
hostname_template: "x"
contours: [lkoz]
groups: []
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for unresolved env var")
	}
}

func TestConfigDuplicateGroup(t *testing.T) {
	t.Setenv("PG_ADMIN_USER", "u")
	t.Setenv("PG_ADMIN_PASSWORD", "p")

	path := writeYAML(t, `
admin:
  username: ${PG_ADMIN_USER}
  password: ${PG_ADMIN_PASSWORD}
hostname_template: "x"
contours: [lkoz]
groups:
  - name: alpha
  - name: alpha
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate group")
	}
}

func TestConfigUnknownContourInOverride(t *testing.T) {
	t.Setenv("PG_ADMIN_USER", "u")
	t.Setenv("PG_ADMIN_PASSWORD", "p")

	path := writeYAML(t, `
admin:
  username: ${PG_ADMIN_USER}
  password: ${PG_ADMIN_PASSWORD}
hostname_template: "x"
contours: [lkoz]
groups:
  - name: alpha
    overrides:
      unknown_contour: some-host.example.com
`)
	_, err := config.Load(path)
	if err == nil {
		t.Fatal("expected error for unknown contour in override")
	}
}

func TestConfigFileNotFound(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
