//go:build !wasm && prod

package config

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/router"
)

func validateAuthorize(cfg ServerConfig) error {
	if cfg.Authorize == nil {
		return fmt.Err("production: Authorize cannot be nil (fail-open protection)")
	}
	return nil
}

// introspectionAccess: en producción /_routes exige api_explorer:r (AS-8).
func introspectionAccess(r router.Route) { r.Requires(ResourceAPIExplorer, model.Read) }
