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
	"webtyp.com/router"
	"webtyp.com/rpc"
	"webtyp.com/storage/mem"

	servicecatalog "github.com/veltylabs/service_catalog"

	"github.com/veltylabs/mjosefa-cms/config"
	"github.com/veltylabs/mjosefa-cms/modules"
)

// productionServer arma el servidor como main(): ProductionDeps sobre una base en memoria.
func productionServer(t *testing.T) *httptest.Server {
	t.Helper()
	db := orm.New(mem.New())
	cfg, err := config.ProductionDeps(db, config.ServerConfig{Logger: t.Log})
	if err != nil {
		t.Fatalf("ProductionDeps: %v", err)
	}
	cfg.Port = "0"
	cfg.OpenDB = func(string) (*orm.DB, error) { return db, nil }
	srv, err := config.BuildServer(cfg)
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts
}

func post(t *testing.T, url, body string, cookies []*http.Cookie) (int, string) {
	t.Helper()
	req, _ := http.NewRequest("POST", url, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

// AS-4: con config.MCPTools() vacío, un agente no ve ninguna herramienta.
func TestMCPClosedByDefault(t *testing.T) {
	if n := len(config.MCPTools()); n != 0 {
		t.Fatalf("MCPTools() = %d tools; this test checks the closed default", n)
	}
	ts := productionServer(t)

	status, body := post(t, ts.URL+mcp.MCPPath, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, nil)
	if status != http.StatusOK {
		t.Fatalf("tools/list: status %d, body %s", status, body)
	}
	if !strings.Contains(body, `"tools":[]`) {
		t.Fatalf("tools/list must list zero tools, got %s", body)
	}
}

// AS-1/AS-2: una lectura de service_catalog responde por POST /api/<módulo>/<op>: 200 con sesión,
// 403 sin ella (contrato de webtyp.com/router: un anónimo en una ruta no pública recibe 403; el
// 401 queda para el desafío de MFA).
func TestRPCMounted_ItemCatalogRead(t *testing.T) {
	ts := productionServer(t)
	path := ts.URL + rpc.DefaultPrefix + "/" + servicecatalog.ModelName + "/" + servicecatalog.OpListServices
	args := `{"tenant_id":"` + modules.TenantID + `"}`

	if status, body := post(t, path, args, nil); status != http.StatusForbidden {
		t.Fatalf("without a session: status %d (body %s), want 403", status, body)
	}

	session := seedAdminThroughSetup(t, ts.URL, "12345678-5", "Admin")
	if status, body := post(t, path, args, session); status != http.StatusOK {
		t.Fatalf("with a session: status %d (body %s), want 200", status, body)
	}
}

// AS-7/AS-8: en desarrollo /_routes es público y describe las operaciones publicadas por rpc.
func TestIntrospectionListsRPCRoutes(t *testing.T) {
	ts := productionServer(t)

	resp, err := http.Get(ts.URL + router.IntrospectionPath)
	if err != nil {
		t.Fatalf("GET %s: %v", router.IntrospectionPath, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %s", router.IntrospectionPath, resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"`+rpc.DefaultPrefix+"/") {
		t.Fatalf("%s lists no %s route: %s", router.IntrospectionPath, rpc.DefaultPrefix, body)
	}
}
