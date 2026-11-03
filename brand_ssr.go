//go:build !wasm

package appkit

import "webtyp.com/html"

// RenderPages is the access screen served at "/" before any wasm loads.
func (b Brand) RenderPages() []html.Page {
	if b.Name == "" {
		return nil
	}
	return loginPages(b)
}

// RenderHTML is what #app shows while the browser binary downloads.
func (b Brand) RenderHTML() string {
	if b.Name == "" {
		return ""
	}
	return loadingHTML()
}
