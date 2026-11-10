//go:build !wasm

package appkit

// RenderHTML is what #app shows while the browser binary downloads.
func (a App) RenderHTML() string {
	if a.Brand.Name == "" {
		return ""
	}
	return loadingHTML()
}
