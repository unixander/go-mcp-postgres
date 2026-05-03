package resolver

import (
	"fmt"
	"strings"

	"mcp-postgres/internal/config"
	mcperrors "mcp-postgres/internal/errors"
)

type Resolver struct {
	cfg      *config.Config
	groupMap map[string]*config.GroupConfig
}

func New(cfg *config.Config) *Resolver {
	return &Resolver{cfg: cfg, groupMap: cfg.GroupMap()}
}

func (r *Resolver) ResolveHost(group, contour string) (string, error) {
	g, ok := r.groupMap[group]
	if !ok {
		return "", &mcperrors.GroupNotFoundError{Group: group}
	}
	if host, ok := g.Overrides[contour]; ok {
		return host, nil
	}
	host := strings.NewReplacer("{group}", group, "{contour}", contour).
		Replace(r.cfg.HostnameTemplate)
	return host, nil
}

func (r *Resolver) ResolveDSN(group, contour, database string) (string, error) {
	host, err := r.ResolveHost(group, contour)
	if err != nil {
		return "", err
	}
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?connect_timeout=%d",
		r.cfg.Admin.Username,
		r.cfg.Admin.Password,
		host,
		r.cfg.Admin.Port,
		database,
		r.cfg.Defaults.ConnectTimeoutS,
	)
	return dsn, nil
}

func (r *Resolver) Group(name string) (*config.GroupConfig, bool) {
	g, ok := r.groupMap[name]
	return g, ok
}

func (r *Resolver) Config() *config.Config { return r.cfg }
