package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPoolIdleTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		yaml    string
		want    int
		invalid bool
	}{
		{name: "default", yaml: "{}", want: 300},
		{name: "configured", yaml: "defaults:\n  pool_max_idle_time_s: 60", want: 60},
		{name: "zero", yaml: "defaults:\n  pool_max_idle_time_s: 0", invalid: true},
		{name: "negative", yaml: "defaults:\n  pool_max_idle_time_s: -1", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if tc.invalid {
				if err == nil {
					t.Fatal("expected invalid idle timeout to be rejected")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Defaults.PoolMaxIdleTimeS != tc.want {
				t.Fatalf("idle timeout = %d, want %d", cfg.Defaults.PoolMaxIdleTimeS, tc.want)
			}
		})
	}
}
