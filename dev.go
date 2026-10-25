//go:build !wasm && !prod

package appkit

import (
	"webtyp.com/model"
	"webtyp.com/router"
)

func validateAuthorize(cfg ServerConfig) error {
	return nil
}

// DevAllowAll provee un opt-in explícito para que desarrollo evite la autorización.
func DevAllowAll() model.Authorizer {
	return func(userID string, resource model.Resource, action model.Action) bool {
		return true
	}
}

// introspectionAccess: en desarrollo /_routes es público.
func introspectionAccess(r router.Route) { r.Public() }
