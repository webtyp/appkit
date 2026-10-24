//go:build !wasm

// Package tests aloja el carril de integración de servidor: httptest contra
// config.BuildServer, sin Postgres. Entrar un módulo nuevo significa agregar
// su propio <modulo>_setup_test.go y <modulo>_<dominio>_test.go — este
// archivo y app/ no se tocan.
package tests

import (
	"bytes"
	"net/http"
	"testing"

	"webtyp.com/auth"
	"webtyp.com/context"
	"webtyp.com/mcp"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/router"

	"github.com/veltylabs/mjosefa-cms/config"
)

// fakeAuthn identifica siempre al llamador como testUserID — un doble del
// middleware real de validación de sesión de tinywasm/user.
func fakeAuthn(userID string) router.Middleware {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(ctx router.Context) {
			ctx.SetUserID(userID)
			next(ctx)
		}
	}
}

const testUserID = "test-user"

// fakeToolProvider expone una tool de lectura y una de creación bajo el
// mismo resource, para que una sola función Authorize pueda permitir una y
// negar la otra — superficie suficiente para probar que la identidad llega
// al RBAC por-tool de MCP.
type fakeToolProvider struct{}

func (fakeToolProvider) Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "read_thing",
			Description: "read",
			Resource:    "service",
			Action:      model.Read,
			Execute: func(_ *context.Context, _ mcp.Request) (*mcp.Result, error) {
				return mcp.Text("ok"), nil
			},
		},
		{
			Name:        "create_thing",
			Description: "create",
			Resource:    "service",
			Action:      model.Create,
			Execute: func(_ *context.Context, _ mcp.Request) (*mcp.Result, error) {
				return mcp.Text("created"), nil
			},
		},
	}
}

// fakeModule es el módulo de operaciones de prueba: el contrato del navegador (rpc) contra el
// servidor armado, sin depender de un módulo de dominio.
const (
	fakeModule  = "fake"
	opReadThing = "read_thing"
	opMakeThing = "create_thing"
)

// fakeOps publica una lectura y una creación sobre el mismo recurso que fakeToolProvider, para que
// allowReadOnly permita una y niegue la otra. made cuenta las creaciones que llegaron al handler.
type fakeOps struct{ made *int }

func (fakeOps) ModelName() string { return fakeModule }

func (f fakeOps) MountOperations(reg router.OperationRegistry) {
	reg.Operation(opReadThing, func(ctx router.Context) {
		ctx.Write([]byte("{}"))
	}).Requires("service", model.Read)
	reg.Operation(opMakeThing, func(ctx router.Context) {
		*f.made++
		ctx.Write([]byte("{}"))
	}).Requires("service", model.Create)
}

// allowReadOnly autoriza solo acciones 'r' sobre "service" para
// testUserID — un doble RBAC mínimo.
func allowReadOnly(userID string, resource model.Resource, action model.Action) bool {
	return userID == testUserID && resource == "service" && action == model.Read
}

// buildUnderTest conecta config.BuildServer con dobles de test (sin Postgres
// — OpenDB nunca es dereferenciado por el fake provider) y devuelve el
// http.Handler resultante para httptest.
// seedAdminThroughSetup da de alta el primer administrador por la MISMA ruta
// de primera ejecución que usa una persona instalando (auth.PathSetup), en vez
// de sembrarlo por variables de entorno como se hacía antes. Vale la pena que
// los tests pasen por aquí: el alta queda ligada a la IP del cliente de test
// (loopback), que es exactamente el comportamiento que hay que proteger —
// quien instala queda habilitado en la máquina desde la que instaló.
//
// Devuelve las cookies de sesión que el alta emite: el instalador queda dentro,
// así que un test que necesite una sesión de admin no tiene que loguearse de
// nuevo (y de paso no gasta un intento contra el limitador por IP).
func seedAdminThroughSetup(t *testing.T, baseURL, code, name string) []*http.Cookie {
	t.Helper()
	body := []byte(`{"code":"` + code + `","name":"` + name + `"}`)
	req, err := http.NewRequest("POST", baseURL+auth.PathSetup, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build setup request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", auth.PathSetup, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 302 {
		t.Fatalf("POST %s: status = %d, want 302 (first-run setup should create the admin)", auth.PathSetup, resp.StatusCode)
	}
	return resp.Cookies()
}

func buildUnderTest(t *testing.T, cfg config.ServerConfig) http.Handler {
	t.Helper()
	cfg.Port = "0"
	cfg.Logger = t.Log
	if cfg.OpenDB == nil {
		cfg.OpenDB = func(string) (*orm.DB, error) { return nil, nil }
	}
	srv, err := config.BuildServer(cfg)
	if err != nil {
		t.Fatalf("config.BuildServer: %v", err)
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("srv.Handler: %v", err)
	}
	return h
}
