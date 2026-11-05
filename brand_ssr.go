//go:build !wasm

package appkit

// RenderHTML is what #app shows while the browser binary downloads.
func (b Brand) RenderHTML() string {
	if b.Name == "" {
		return ""
	}
	return loadingHTML()
}
