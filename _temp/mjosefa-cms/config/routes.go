//go:build !wasm

package config

import "webtyp.com/router"

// Register monta cada ruta que responde el servidor de mjosefa-cms.
//
// Ejemplo: config.Register(srv.Router())
func Register(r router.Router) {
}
