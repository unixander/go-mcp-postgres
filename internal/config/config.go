package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Admin struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Port     int    `mapstructure:"port"`
}

type GroupConfig struct {
	Name      string            `mapstructure:"name"`
	Writable  bool              `mapstructure:"writable"`
	Overrides map[string]string `mapstructure:"overrides"`
}

type Defaults struct {
	StatementTimeoutMs         int    `mapstructure:"statement_timeout_ms"`
	MaxRows                    int    `mapstructure:"max_rows"`
	PoolMinSize                int    `mapstructure:"pool_min_size"`
	PoolMaxSize                int    `mapstructure:"pool_max_size"`
	PoolMaxIdleTimeS           int    `mapstructure:"pool_max_idle_time_s"`
	ConnectTimeoutS            int    `mapstructure:"connect_timeout_s"`
	DefaultDatabase            string `mapstructure:"default_database"`
	IdleInTransactionTimeoutMs int    `mapstructure:"idle_in_transaction_timeout_ms"`
}

type Config struct {
	Admin            Admin         `mapstructure:"admin"`
	HostnameTemplate string        `mapstructure:"hostname_template"`
	Contours         []string      `mapstructure:"contours"`
	Groups           []GroupConfig `mapstructure:"groups"`
	Defaults         Defaults      `mapstructure:"defaults"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	expanded := os.ExpandEnv(string(raw))

	// Fail fast on unexpanded ${VAR} patterns.
	if idx := strings.Index(expanded, "${"); idx != -1 {
		end := strings.Index(expanded[idx:], "}")
		varName := ""
		if end > 0 {
			varName = expanded[idx+2 : idx+end]
		}
		return nil, fmt.Errorf("unresolved env variable: ${%s}", varName)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(strings.NewReader(expanded)); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set defaults.
	v.SetDefault("admin.port", 5432)
	v.SetDefault("defaults.statement_timeout_ms", 30000)
	v.SetDefault("defaults.max_rows", 1000)
	v.SetDefault("defaults.pool_min_size", 0)
	v.SetDefault("defaults.pool_max_size", 4)
	v.SetDefault("defaults.pool_max_idle_time_s", 300)
	v.SetDefault("defaults.connect_timeout_s", 5)
	v.SetDefault("defaults.default_database", "postgres")
	v.SetDefault("defaults.idle_in_transaction_timeout_ms", 30000)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validate(cfg *Config) error {
	if cfg.Defaults.PoolMaxIdleTimeS <= 0 {
		return fmt.Errorf("defaults.pool_max_idle_time_s must be positive")
	}
	validContours := make(map[string]bool, len(cfg.Contours))
	for _, c := range cfg.Contours {
		validContours[c] = true
	}

	seen := make(map[string]bool, len(cfg.Groups))
	for _, g := range cfg.Groups {
		if seen[g.Name] {
			return fmt.Errorf("duplicate group name: %q", g.Name)
		}
		seen[g.Name] = true

		for contour := range g.Overrides {
			if !validContours[contour] {
				return fmt.Errorf("group %q: override references unknown contour %q", g.Name, contour)
			}
		}
	}
	return nil
}

// GroupMap returns a map[name]GroupConfig for fast lookup.
func (c *Config) GroupMap() map[string]*GroupConfig {
	m := make(map[string]*GroupConfig, len(c.Groups))
	for i := range c.Groups {
		m[c.Groups[i].Name] = &c.Groups[i]
	}
	return m
}
