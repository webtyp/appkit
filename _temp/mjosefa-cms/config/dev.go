//go:build !wasm && !prod

package config

import (
	"webtyp.com/model"
	"webtyp.com/router"
)

func validateAuthorize(cfg ServerConfig) error {
	return nil
}

// DevAllowAll provee un opt-in explícito para que desarrollo evite la autorización.
// Ejemplo: cfg.Authorize = config.DevAllowAll()
func DevAllowAll() model.Authorizer {
	return func(userID string, resource model.Resource, action model.Action) bool {
		return true
	}
}

// introspectionAccess: en desarrollo /_routes es público (AS-8).
func introspectionAccess(r router.Route) { r.Public() }
