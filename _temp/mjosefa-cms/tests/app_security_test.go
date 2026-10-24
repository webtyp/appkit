//go:build !wasm

package tests

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webtyp.com/mcp"
	"webtyp.com/orm"

	"github.com/veltylabs/mjosefa-cms/config"
)

// postMCP envía una petición cruda JSON-RPC tools/call y devuelve el cuerpo
// de la respuesta. mcp responde los errores JSON-RPC (incluidas las
// denegaciones RBAC) con HTTP 200 y el error incrustado en el cuerpo — es el
// protocolo JSON-RPC, no una falla de transporte — así que los llamadores
// asertan sobre el contenido del cuerpo, no sobre el status code.
func postMCP(t *testing.T, url, toolName string) string {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":"1","method":"tools/call","params":{"name":"` + toolName + `","arguments":"{}"}}`
	resp, err := http.Post(url+mcp.MCPPath, "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST %s: %v", mcp.MCPPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (JSON-RPC errors are still HTTP 200)", resp.StatusCode)
	}
	out, _ := io.ReadAll(resp.Body)
	return string(out)
}

// TestServiceCatalog_Read_Allowed prueba que la identidad seteada por Authn
// llega al RBAC por-tool de MCP y que una acción permitida tiene éxito.
func TestServiceCatalog_Read_Allowed(t *testing.T) {
	h := buildUnderTest(t, config.ServerConfig{
		Providers: func(*orm.DB) []mcp.ToolProvider { return []mcp.ToolProvider{fakeToolProvider{}} },
		Authn:     fakeAuthn(testUserID),
		Authorize: allowReadOnly,
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	out := postMCP(t, ts.URL, "read_thing")
	if strings.Contains(out, `"error"`) {
		t.Fatalf("expected allowed call to succeed, got: %s", out)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("expected result content %q, got: %s", "ok", out)
	}
}

// TestServiceCatalog_Create_Denied prueba que la misma identidad es negada
// para una acción que el doble RBAC no concede — el RBAC por-tool corre de
// verdad, no deja pasar todo silenciosamente.
func TestServiceCatalog_Create_Denied(t *testing.T) {
	h := buildUnderTest(t, config.ServerConfig{
		Providers: func(*orm.DB) []mcp.ToolProvider { return []mcp.ToolProvider{fakeToolProvider{}} },
		Authn:     fakeAuthn(testUserID),
		Authorize: allowReadOnly,
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	out := postMCP(t, ts.URL, "create_thing")
	if !strings.Contains(out, "forbidden") {
		t.Fatalf("expected forbidden error, got: %s", out)
	}
}

// TestDefaultDeny_NoAuthorizeConfigured es D1: Deps{} (zero-value) jamás
// debe significar "permitir todo" — toda llamada a tool es negada hasta que
// Authorize quede conectado.
func TestDefaultDeny_NoAuthorizeConfigured(t *testing.T) {
	h := buildUnderTest(t, config.ServerConfig{
		Providers: func(*orm.DB) []mcp.ToolProvider { return []mcp.ToolProvider{fakeToolProvider{}} },
		Authn:     fakeAuthn(testUserID),
		// Authorize dejado nil a propósito.
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	out := postMCP(t, ts.URL, "read_thing")
	if !strings.Contains(out, "forbidden") {
		t.Fatalf("expected default-deny to reject an unauthorized-config call, got: %s", out)
	}
}

// TestAnonymous_DeniedByDefault es D4: sin Authn, el llamador no tiene
// identidad (UserID() == ""). /mcp es el envelope público, así que la
// petición HTTP tiene éxito con 200, pero la llamada a tool interna y
// protegida es negada con un error JSON-RPC.
func TestAnonymous_DeniedByDefault(t *testing.T) {
	h := buildUnderTest(t, config.ServerConfig{
		Providers: func(*orm.DB) []mcp.ToolProvider { return []mcp.ToolProvider{fakeToolProvider{}} },
		Authorize: allowReadOnly,
		// Authn dejado nil a propósito — nunca se setea identidad alguna.
	})
	ts := httptest.NewServer(h)
	defer ts.Close()

	out := postMCP(t, ts.URL, "read_thing")
	if !strings.Contains(out, "forbidden") {
		t.Fatalf("expected anonymous caller to receive forbidden error on guarded tool, got: %s", out)
	}
}
