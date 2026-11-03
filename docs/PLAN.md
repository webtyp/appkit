---
PLAN: "feat!: one App value for both binaries — Module with Server and Browser parts, LoginByRUT, renamed fields"
TAG: v0.2.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 2227120075361327736
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go get webtyp.com/access@latest webtyp.com/rbac@latest webtyp.com/auth@latest
```

## Why

Today an app declares two values and lists every module twice (`veltylabs/mjosefa-cms`):

```go
// config/app.go (!browser)
var App = appkit.App{Name: TenantID, Tenant: TenantID, Timezone: Timezone, Open: postgres.Open,
	Modules: []appkit.Module{{Name: "auth", Migrate: authority.Migrate}, {Name: "rbac", Migrate: rbac.Migrate},
		{Name: access.ModelName, Migrate: accessmigrate.Migrate}, catalog.Module, devices.Module, staff.Module, rooms.Module},
	Auth: buildAuth /* 279 lines in config/auth.go */, MCPTools: MCPTools(), Register: Register, Logger: Logger}
// config/browser.go (browser) — web/browser/main.go (wasm only) uses it, so gopls reports "undefined: config.Browser"
var Browser = appkit.Browser{Brand: Brand, Screens: modules.Screens, DefaultScreen: servicecatalogui.ID, IdleSeconds: 1800}
```

After this plan, one value with no build tag, used by both binaries:

```go
// config/app.go — no build tag
var App = appkit.App{
	TenantID: "mjosefa-cms",
	Timezone: "America/Santiago",
	Brand:    Brand,
	Modules:  []appkit.Module{catalog.Module, devices.Module, staff.Module, rooms.Module},
	Login:    appkit.LoginByRUT{},
	Server:   server, // config/server.go (!browser): appkit.AppServer{Database: postgres.Open, MCPTools: ...}
	Log:      fmt.Println,
}
// web/browser/main.go: func main() { appkit.RunBrowser(config.App) }
```

**The rule that removes the gopls error**: every symbol without a build tag exists under every tag combination. A part
that belongs to one binary is a type that is a real struct under its tag and an empty struct under the other.

## Ecosystem search

`appkit` already is the composition piece (`App`, `Module`, `Find`, `BuildServer`, `RunBrowser`). The 279 lines moved in
are the RUT login wiring every veltylabs app repeats; the pieces are reused, not re-written: `webtyp.com/auth/authority`
(users, sessions), `webtyp.com/auth/trusted_ip` (login by RUT + first admin), `webtyp.com/rbac`, `webtyp.com/access`
(`EnsureRoleFor`). The devices and staff record of the first admin stay in `veltylabs/staff_manager`, reached through the
interface `FirstAdminRegistrar` with `Find` (appkit never imports veltylabs). **Extend** appkit.

## Design gate

1. **Prior art.** Rails `config/application.rb` and Django `settings.INSTALLED_APPS` list each app/module once
   (https://guides.rubyonrails.org/configuring.html, https://docs.djangoproject.com/en/5.1/ref/settings/#installed-apps);
   Phoenix declares one `Endpoint` per app (https://hexdocs.pm/phoenix/Phoenix.Endpoint.html). One list, one place.
   Here the list must compile into two binaries, hence the per-tag parts.
2. **Names.** `TenantID`, `Timezone`, `Brand`, `Modules`, `Login`, `Server`, `Log`; `AppServer{Database, MCPTools, Routes}`;
   `Module{Name, Server, Browser}`, `ModuleServer{Migrate, Build}`, `ModuleBrowser{Screens}`; `LoginByRUT{IdleMinutes}`.
3. **Ledger.** Concepts −2 (`Browser`, `Auth`), files the app touches −5, ways to list a module 2 → 1.
4. **Where.** All in appkit: it is the only piece that knows the app as a whole.
5. **Deletes.** `Browser`, `App.Auth`, `Auth` (exported), `App.Name`, `App.Tenant`, `App.Open`, `App.Register`, `App.Logger`,
   `Module.Migrate`/`Module.Build` at top level, `DefaultScreen`, exported `LoginPages`/`LoadingHTML`.

## APIs used

| Symbol | Signature | File |
|---|---|---|
| `authority.New` | `func New(db *orm.DB, cfg auth.Config) (*Module, error)` | `auth/authority/module.go` |
| `(*authority.Module).CreateUser` | `(email, name, phone string) (auth.User, error)` | `auth/authority/ports.go` |
| `(*authority.Module).RegisterLAN` | `(userID, rut string) error` | `auth/authority/credentials_lan.go` |
| `(*authority.Module).HasIdentity` | `(provider string) (bool, error)` | `auth/authority/identities.go` |
| `(*authority.Module).Enable` / `SetLog` / `Authenticate` | `(auths ...auth.Authenticator)` / `(fn func(...any))` / `() router.Middleware` | `auth/authority/module.go`, `middleware.go` |
| `authority.Migrate`, `rbac.Migrate`, `accessmigrate.Migrate` | `func(conn ddl.Execer, c ddl.Compiler) error` | `auth/authority/migrate.go`, `rbac/migrate.go`, `access/migrate/migrate.go` |
| `trustedip.New` | `func New(store auth.IdentityStore, trusted auth.TrustedIPStore, sessions auth.SessionIssuer, notify auth.SecurityNotifier, trustProxy bool, opts ...Option) *Authenticator` | `auth/trusted_ip/trusted_ip.go` |
| `trustedip.WithRateLimit` / `WithFirstAdminSetup` | `(l auth.RateLimiter) Option` / `(reg auth.ProviderRegistry, create func(code, name, ip string) error) Option` | same |
| `auth.NewIPLimiter` | `func NewIPLimiter(maxAttempts int, window, block time.Unix) *IPLimiter` | `auth/limiter.go` |
| `auth.TrustedIPStore` | `interface{ IsTrustedIP(userID, ip string) bool }` | `auth/user.go` |
| `rbac.New` | `func New(db *orm.DB) (*Service, error)` | `rbac` |
| `(*rbac.Service)` | `GetRoleByCode(projectID string, code model.RoleCode) (*rbac.Role, error)`, `CreatePermission(projectID, id, name string, resource model.Resource, action model.Action) error`, `AssignPermission(projectID, roleID, permissionID string) error`, `Can(projectID, userID string, r model.Resource, a model.Action) bool` | `rbac/rbac.go` |
| `access.New` / `EnsureRoleFor` | `func New(d access.Deps) (*Module, error)` / `(code model.RoleCode, name, description string, userIDs []string) error` | `access/module.go` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stages

### Stage 1 — types (`app.go` no tag, `module_server.go`, `module_browser.go`, `app_server.go`, `app_browser.go`)

`app.go` (no build tag; delete the old content and `module.go`):

```go
package appkit

// App is the whole application, for both binaries: web/server and web/browser read the same value.
//
//	var App = appkit.App{TenantID: "acme", Brand: Brand, Modules: []appkit.Module{catalog.Module}, Login: appkit.LoginByRUT{}, Server: server}
type App struct {
	TenantID string       // the installation: every module's tenant, the rbac project and the MCP server name
	Timezone string       // e.g. "America/Santiago"
	Brand    Brand        // name and logo on the access screen and the chassis
	Modules  []Module     // build, migration and menu order; the first screen is the one opened first
	Login    Login        // how people enter; nil = nobody: every operation is denied
	Server   AppServer    // what only the server binary needs (declared in a !browser file of the app)
	Log      func(...any) // nil = discard
}

// Module is one domain module as an app lists it in App.Modules, with its server part and its screens.
//
//	var Module = appkit.Module{Name: "service_catalog", Server: server, Browser: browser}
type Module struct {
	Name    string        // identity: migration step name, error prefix
	Server  ModuleServer  // a !browser file of the module fills it
	Browser ModuleBrowser // a browser file of the module fills it
}

// Login is how people enter the app. Only appkit implements it: today LoginByRUT.
type Login interface{ loginMethod() }

// LoginByRUT is entry by RUT from a device the installation knows (trusted IP), with first-administrator setup.
//
//	Login: appkit.LoginByRUT{}
type LoginByRUT struct {
	IdleMinutes int // session idle lock; 0 = SESSION_IDLE_MINUTES or 30
}

func (LoginByRUT) loginMethod() {}
```

`module_server.go` (`//go:build !browser`): `ModuleServer{Migrate func(conn ddl.Execer, compiler ddl.Compiler) error; Build func(d Deps) (model.ModuleNaming, error)}`,
`type ModuleBrowser struct{}`, and **move here unchanged** from the old `module.go`: `Deps` (rename field `Tenant` → `TenantID`),
`Find`, `ErrNoProvider`, `ErrManyProviders`. Add:

```go
// FirstAdminRegistrar is what a module offers to complete the first administrator (its device, its staff record).
// Optional: with no module providing it, only the user and its role are created.
type FirstAdminRegistrar interface {
	RegisterFirstAdmin(userID, code, name, ip string) error
}
```

`module_browser.go` (`//go:build browser`): `type ModuleBrowser struct{ Screens func(caller router.Caller) []view.Screen }` and `type ModuleServer struct{}`.

`app_server.go` (`//go:build !browser`):

```go
// AppServer is what only the server binary needs.
//
//	var server = appkit.AppServer{Database: postgres.Open}
type AppServer struct {
	Database func(dsn string) (storage.Conn, error) // the driver (postgres.Open); nil = no database
	MCPTools []mcp.ToolName                         // operations an agent may call; empty = MCP closed
	Routes   func(r router.Router)                  // the app's own routes; nil = none
}
```

`app_browser.go` (`//go:build browser`): `type AppServer struct{}`.

Change the tag of `serve.go`, `migrate.go` from `!wasm` to `!browser` only if they compile under `GOOS=js GOARCH=wasm`;
otherwise leave `!wasm` (they are never referenced from a tagless file). `run_browser.go`: `//go:build wasm` → `//go:build browser`.

Commit.

### Stage 2 — server and migrations (`serve.go`, `migrate.go`, `login_rut_server.go` new)

- `BuildServer` / `Migrate` read `app.Server.Database` (was `app.Open`), `app.TenantID` (was `Tenant`, and `Name` for MCP),
  `app.Server.MCPTools`, `app.Server.Routes` (into `ServerConfig.Register`), `app.Log`. Error text of `ErrNoOpen` becomes
  `"appkit: DATABASE_URL is set but App.Server.Database is nil — give it the database driver (postgres.Open)"`.
- `Steps(app)`: when `app.Login` is `LoginByRUT`, first the three steps `{"auth", authority.Migrate}`, `{"rbac", rbac.Migrate}`,
  `{access.ModelName, accessmigrate.Migrate}`, then each module with `Server.Migrate != nil`.
- Auth: replace the `app.Auth` call by

```go
var au serverAuth
if d.DB != nil {
	switch l := app.Login.(type) {
	case LoginByRUT:
		if au, err = l.build(d, logger); err != nil {
			return nil, fmt.Errorf("login: %w", err)
		}
	}
}
```

`serverAuth` is the old exported `Auth` struct, renamed unexported (same fields).

`login_rut_server.go` (`//go:build !browser`): port of `config/auth.go` of mjosefa-cms (reproduced at the end of this
plan) with these changes — `ProjectID` → `d.TenantID`; `Logger` → `logger`; constants are appkit's own:

```go
const (
	EnvSessionIdleMinutes     = "SESSION_IDLE_MINUTES"
	defaultSessionIdleMinutes = 30
	maxLoginAttempts          = 3
	loginWindow               = 15 * 60 // seconds
	loginBlock                = 15 * 60
	RoleAdmin                 = model.RoleCode("admin")
	permWildcardAll           = "wildcard:all"
)
```

- `(l LoginByRUT) build(d Deps, logger func(...any)) (serverAuth, error)` = `buildAuth` of the original, with
  idle minutes: `l.IdleMinutes` if > 0, else `SESSION_IDLE_MINUTES` parsed as in the original, else 30.
- The trusted-IP store is `appkit.Find[auth.TrustedIPStore](d)` (was `*staffmanager.Module`); an error is returned as is.
- The first admin, replacing `seedFirstAdmin`, `createDeviceWithIP`, `ensureIPv6Alias`, `adminDevices` (all deleted):

```go
func seedFirstAdmin(d Deps, authMod *authority.Module, rbacSvc *rbac.Service, accessMod *access.Module, code, name, ip string) error {
	user, err := authMod.CreateUser("", name, "")
	if err != nil {
		return err
	}
	if err := authMod.RegisterLAN(user.Id, code); err != nil {
		return err
	}
	switch reg, err := Find[FirstAdminRegistrar](d); {
	case err == nil:
		if err := reg.RegisterFirstAdmin(user.Id, code, name, ip); err != nil {
			return err
		}
	case !errors.Is(err, ErrNoProvider):
		return err
	}
	if err := accessMod.EnsureRoleFor(RoleAdmin, "Admin", "System administrator", []string{user.Id}); err != nil {
		return err
	}
	role, err := rbacSvc.GetRoleByCode(d.TenantID, RoleAdmin)
	if err != nil {
		return err
	}
	if err := rbacSvc.CreatePermission(d.TenantID, permWildcardAll, permWildcardAll, model.Wildcard, model.AllActions); err != nil {
		return err
	}
	return rbacSvc.AssignPermission(d.TenantID, role.Id, permWildcardAll)
}
```

- `SecurityEventLogger` is ported unexported (`securityEventLogger`). Startup log lines in English:
  `"appkit: the database does not answer the administrators query — run go run ./migrate once. Detail:"` and
  `"appkit: no administrator yet — open the app in the browser to complete the first setup"`.

Commit.

### Stage 3 — browser (`run_browser.go`, `browser.go`, `brand_ssr.go` new)

- `RunBrowser(app App)` replaces `RunBrowser(b Browser)`; delete `type Browser`. Screens = in `app.Modules` order, each
  `m.Browser.Screens(caller)` that is non-nil, appended. `platformd.Platform.DefaultID` is left empty (platformd opens the
  first viewable module, `layout/platformd/platformd.go:127`). Idle seconds: `LoginByRUT.IdleMinutes` if > 0, else 30, × 60.
  Brand = `app.Brand`.
- `browser.go`: `LoginPages` → `loginPages`, `LoadingHTML` → `loadingHTML` (unexported).
- `brand_ssr.go` (`//go:build !wasm`): sitec calls these on the app's `var Brand` (a zero Brand renders nothing, so
  sitec's zero-value discovery of the type adds nothing):

```go
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
```

Commit.

### Stage 4 — tests (`tests/app_test.go`, `tests/login_test.go` new)

- Rewrite `tests/app_test.go` to the new shape: `mod(...)` returns `appkit.Module{Name: name, Server: appkit.ModuleServer{Migrate: …, Build: build}}`;
  `appkit.App{TenantID: …, Server: appkit.AppServer{Database: openMem}, Modules: …}`. Every existing test keeps its assertion.
  `TestBuildServer_AuthSeesModules` and `TestBuildServer_AuthOperations` are deleted (`App.Auth` no longer exists).
- New `tests/login_test.go` (real stack on `storage/mem`, as the other tests):

| Test | Setup | Expected |
|---|---|---|
| `TestSteps_LoginByRUTFirst` | `App{Login: LoginByRUT{}, Modules: [mod("a")]}` | `Steps(app)` names in order: `auth`, `rbac`, `access`, `a` |
| `TestSteps_NoLoginNoAuthSteps` | `App{Modules: [mod("a")]}` | names: `a` |
| `TestBuildServer_LoginNeedsTrustedIPStore` | `LoginByRUT{}`, no module providing `auth.TrustedIPStore`, `Database: openMem`, DSN `"x"` | `BuildServer` error wraps `appkit.ErrNoProvider` |
| `TestBuildServer_LoginByRUT` | a module whose Build returns a value with `ModelName()` and `IsTrustedIP(userID, ip string) bool { return true }`; run `RunMigrations` first on the same mem conn | `BuildServer` returns no error; `GET /api/...me` without session is 401 (any existing helper of `server_test.go`) |
| `TestBrand_ZeroRendersNothing` | `appkit.Brand{}` | `RenderPages()` is nil, `RenderHTML()` is `""` |
| `TestBrand_RendersLogin` | `appkit.Brand{Name: "Acme"}` | one page, `Path == "/"`, body contains `Acme` |

Commit.

### Stage 5 — translations and docs

- `langc sync .`: 0 untranslated keys, 0 reuse warnings, 0 identical values.
- `README.md` (≤150 lines, 4 sections): the example is the app of **Why** above plus a module:

```go
// module/module.go (no tag)
var Module = appkit.Module{Name: "service_catalog", Server: server, Browser: browser}
// module/server.go (//go:build !browser)
var server = appkit.ModuleServer{Migrate: migrate.Migrate, Build: build}
var browser appkit.ModuleBrowser
// module/browser.go (//go:build browser)
var server appkit.ModuleServer
var browser = appkit.ModuleBrowser{Screens: ui.Screens}
```

  Say in **Before You Start**: every symbol without a build tag exists under every tag (that is what keeps gopls and both
  binaries compiling).
- `docs/ARCHITECTURE.md`: the per-tag parts, `LoginByRUT` and `FirstAdminRegistrar`.

## Delivery check

`gotest` passes (it builds, vets, and runs stdlib + wasm). A PR that does not compile, or whose tests could not run, is a
failed delivery. Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`, `go mod tidy` first.
`grep -rn "type Browser struct\|Auth  *func\|LoginPages(\b" --include=*.go . | grep -v _test` → only `loginPages`.
Never ask permission; commit after every stage; never `git reset`/`restore`/`stash` your own work. Stuck 30 minutes:
open the PR listing what fails.

| Stage | Files |
|---|---|
| 1 | `app.go`, `module.go` (deleted), `module_server.go`, `module_browser.go`, `app_server.go`, `app_browser.go`, `run_browser.go` |
| 2 | `serve.go`, `migrate.go`, `login_rut_server.go`, `go.mod`, `go.sum` |
| 3 | `run_browser.go`, `browser.go`, `brand_ssr.go` |
| 4 | `tests/app_test.go`, `tests/login_test.go` |
| 5 | `lang.json`, `README.md`, `docs/ARCHITECTURE.md` |

## Source to port — `veltylabs/mjosefa-cms/config/auth.go`

    //go:build !browser
    
    package config
    
    import (
    	"webtyp.com/appkit"
    	"webtyp.com/auth"
    	"webtyp.com/access"
    	"webtyp.com/auth/authority"
    	trustedip "webtyp.com/auth/trusted_ip"
    	"webtyp.com/env"
    	"webtyp.com/events"
    	"webtyp.com/fmt"
    	"webtyp.com/model"
    	"webtyp.com/rbac"
    	"webtyp.com/router"
    	"webtyp.com/time"
    
    	devicemanager "github.com/veltylabs/device_manager"
    	staffmanager "github.com/veltylabs/staff_manager"
    )
    
    const (
    	// ProjectID es el proyecto RBAC de esta app: el mismo tenant de sus módulos.
    	// Ejemplo: rbacSvc.Can(config.ProjectID, userID, "service_catalog", model.Read)
    	ProjectID = TenantID
    
    	// permWildcardAll: identificador de RBAC sembrado en
    	// ProductionDeps. Constante (no literal) — la regla de "sin strings
    	// hardcodeados" del composition root.
    	permWildcardAll = "wildcard:all"
    )
    
    // SecurityEventLogger enruta los eventos de seguridad de auth al logger de la
    // app. Extraído para que el wiring D10 (denegación → log de seguridad) sea
    // testeable sin DB: webtyp.com/auth publica un auth.SecurityEvent en
    // auth.TopicSecurity al denegar (su propio comportamiento ya testado en
    // webtyp.com/auth); ProductionDeps lo suscribe al broker de eventos. Esto es
    // solo la mitad de la app — que el evento llegue a cfg.Logger con su resource.
    //
    // Ejemplo: logSecurity := config.SecurityEventLogger(cfg.Logger)
    func SecurityEventLogger(logger func(...any)) func(auth.SecurityEvent) {
    	return func(e auth.SecurityEvent) {
    		// DEV_AUTOLOGIN no es un incidente: authority ya escribe su propia línea
    		// ("auth: DEV_AUTOLOGIN opened a session for user …").
    		if logger == nil || e.Type == auth.EventDevAutologin {
    			return
    		}
    		args := []any{"app: SECURITY EVENT — type:", e.Type} // SecurityEventType prints its name (Stringer)
    		for _, f := range []struct{ label, value string }{
    			{"userID:", e.UserID},
    			{"resource:", e.Resource},
    			{"IP:", e.IP},
    		} {
    			if f.value != "" {
    				args = append(args, f.label, f.value)
    			}
    		}
    		logger(args...)
    	}
    }
    
    // lanInterfaceLabel nombra la interfaz que recibe un equipo cuando se registra
    // desde la IP con la que inicia sesión.
    const lanInterfaceLabel = "lan"
    
    // adminDeviceLocation es dónde se registra el equipo del instalador.
    const adminDeviceLocation = "Oficina"
    
    // createDeviceWithIP registra un equipo y le da una interfaz "lan" con esa IP.
    // Sin MAC a propósito: el servidor no puede conocer la MAC de quien se conecta
    // (y los alias de loopback no tienen), así que esta interfaz sirve para el
    // login por IP y no se envía al router (device_manager solo entrega al router
    // las interfaces con MAC).
    func createDeviceWithIP(devMod adminDevices, name, ip string) (devicemanager.Device, error) {
    	dev, err := devMod.CreateDevice(devicemanager.Device{
    		TenantId: ProjectID, Name: name,
    		Type: devicemanager.DeviceTypeComputer, IsActive: true, Location: adminDeviceLocation,
    	})
    	if err != nil {
    		return devicemanager.Device{}, err
    	}
    	if _, err := devMod.SaveNetworkInterface(devicemanager.NetworkInterface{
    		TenantId: ProjectID, DeviceId: dev.Id, Label: lanInterfaceLabel, Ip: ip,
    	}); err != nil {
    		return devicemanager.Device{}, err
    	}
    	return dev, nil
    }
    
    // ensureIPv6Alias registra (o reutiliza) un dispositivo en "::1" y lo asigna
    // al mismo miembro que ip, cuando ip es "127.0.0.1" — un mismo navegador
    // local puede conectarse por cualquiera de los dos según cómo resuelva
    // "localhost". Es una decisión de despliegue LAN de esta app, no un
    // workaround de un bug de librería (ese vive en auth.ClientIP, ya corregido
    // río arriba), así que se queda aquí, no en una librería.
    func ensureIPv6Alias(devMod adminDevices, staffMod *staffmanager.Module, member staffmanager.StaffMember, ip, name string) error {
    	// Simétrico a propósito: el alta puede llegar por cualquiera de las dos
    	// caras del loopback (el navegador resuelve "localhost" a ::1, un curl a
    	// 127.0.0.1), y registrar solo la que llegó deja a quien instaló fuera si
    	// después entra por la otra. Fuera del loopback no aplica: una IP de LAN
    	// es una sola.
    	var alias string
    	switch ip {
    	case "127.0.0.1":
    		alias = "::1"
    	case "::1":
    		alias = "127.0.0.1"
    	default:
    		return nil
    	}
    	deviceID, ok := devMod.FindByIP(alias)
    	if !ok {
    		// Sin ":" en Name — el charset de input.Text() no lo permite y
    		// device_manager valida Name contra él.
    		dev6, err := createDeviceWithIP(devMod, name+" (admin, loopback)", alias)
    		if err != nil {
    			return err
    		}
    		deviceID = dev6.Id
    	}
    	return staffMod.AssignDevice(member.Id, deviceID)
    }
    
    // seedFirstAdmin es lo que esta app entiende por "crear el administrador":
    // el usuario de auth, su identidad LAN, el equipo desde el que se dio de alta,
    // su ficha de funcionario, la asignación entre ambos, y el rol admin de RBAC.
    // webtyp.com/auth no sabe nada de equipos ni de funcionarios — por eso recibe
    // esto como callback (trustedip.WithFirstAdminSetup) en vez de intentar
    // hacerlo él.
    //
    // ip es la IP de quien está haciendo el alta: ese equipo queda registrado y
    // asignado, así que el instalador termina habilitado en la máquina donde está
    // parado. No se pide por configuración porque no hay nada que preguntar.
    func seedFirstAdmin(
    	authMod *authority.Module,
    	rbacSvc *rbac.Service,
    	accessMod *access.Module,
    	staffMod *staffmanager.Module,
    	devMod adminDevices,
    	code, name, ip string,
    ) error {
    	// Usuario sin correo (decisión #8 del master plan). Es seguro desde que
    	// webtyp.com/postgres v0.4.13+ lee una columna email NULL como "" en vez de
    	// fallar.
    	adminUser, err := authMod.CreateUser("", name, "")
    	if err != nil {
    		return err
    	}
    	if err := authMod.RegisterLAN(adminUser.Id, code); err != nil {
    		return err
    	}
    	dev, err := createDeviceWithIP(devMod, name+" (admin)", ip)
    	if err != nil {
    		return err
    	}
    	member, err := staffMod.UpsertStaff(staffmanager.StaffMember{
    		TenantId: ProjectID, UserId: adminUser.Id, Rut: code, Name: name, IsActive: true,
    	})
    	if err != nil {
    		return err
    	}
    	if err := staffMod.AssignDevice(member.Id, dev.Id); err != nil {
    		return err
    	}
    	if err := ensureIPv6Alias(devMod, staffMod, member, ip, name); err != nil {
    		return err
    	}
    
    	// El rol y su asignación los hace access (por id: el personal no tiene correo); el permiso
    	// comodín es política de esta app, por eso sigue aquí.
    	if err := accessMod.EnsureRoleFor(RoleAdmin, "Admin", "System administrator", []string{adminUser.Id}); err != nil {
    		return err
    	}
    	role, err := rbacSvc.GetRoleByCode(ProjectID, RoleAdmin)
    	if err != nil {
    		return err
    	}
    	if err := rbacSvc.CreatePermission(ProjectID, permWildcardAll, permWildcardAll, model.Wildcard, model.AllActions); err != nil {
    		return err
    	}
    	return rbacSvc.AssignPermission(ProjectID, role.Id, permWildcardAll)
    }
    
    // adminDevices es lo que el alta del primer administrador usa del módulo de equipos
    // (el valor que construye devices.Module).
    type adminDevices interface {
    	CreateDevice(d devicemanager.Device) (devicemanager.Device, error)
    	SaveNetworkInterface(ni devicemanager.NetworkInterface) (devicemanager.NetworkInterface, error)
    	FindByIP(ip string) (string, bool)
    }
    
    // eventBus es el broker en proceso que appkit entrega: además de publicar, permite suscribirse.
    type eventBus interface {
    	Subscribe(topic string, h events.Handler) events.Subscription
    }
    
    // buildAuth arma auth, rbac y access sobre la base de appkit y conecta el alta del
    // primer administrador con los módulos de equipos y personal. appkit lo llama
    // después de construir todos los módulos.
    //
    // Ejemplo: appkit.App{Auth: buildAuth}
    func buildAuth(d appkit.Deps) (appkit.Auth, error) {
    	if bus, ok := d.Events.(eventBus); ok {
    		logSecurityEvent := SecurityEventLogger(Logger)
    		bus.Subscribe(auth.TopicSecurity, func(e events.Event) {
    			if se, ok := e.Payload.(*auth.SecurityEvent); ok {
    				logSecurityEvent(*se)
    			}
    		})
    	}
    
    	rbacSvc, err := rbac.New(d.DB)
    	if err != nil {
    		return appkit.Auth{}, err
    	}
    
    	idleMinutes := DefaultSessionIdleMinutes
    	if v := env.Get(EnvSessionIdleMinutes); v != "" {
    		if n, err := fmt.Convert(v).Int(); err == nil && n > 0 {
    			idleMinutes = n
    		} else {
    			Logger("app: Warning: invalid " + EnvSessionIdleMinutes + ", using default 30")
    		}
    	}
    
    	authMod, err := authority.New(d.DB, auth.Config{
    		IDs:         d.IDs,
    		Events:      d.Events,
    		IdleTTL:     time.Unix(idleMinutes) * time.Minute,
    		Permissions: &auth.Permissions{Grants: rbacSvc, ProjectID: ProjectID},
    	})
    	if err != nil {
    		return appkit.Auth{}, err
    	}
    	authMod.SetLog(Logger)
    
    	accessMod, err := access.New(access.Deps{
    		DB: d.DB, IDs: d.IDs, Users: authMod, Roles: rbacSvc, ProjectID: ProjectID,
    		// La administración de roles se hace en la clínica, entrando con RUT (un factor).
    		Assurance: model.AssuranceSingleFactor,
    	})
    	if err != nil {
    		return appkit.Auth{}, err
    	}
    
    	staffMod, err := appkit.Find[*staffmanager.Module](d)
    	if err != nil {
    		return appkit.Auth{}, err
    	}
    	devMod, err := appkit.Find[adminDevices](d)
    	if err != nil {
    		return appkit.Auth{}, err
    	}
    
    	loginLimiter := auth.NewIPLimiter(MaxLoginAttempts, LoginWindowSeconds, LoginBlockSeconds)
    	authMod.Enable(trustedip.New(authMod, staffMod, authMod, authMod, false,
    		trustedip.WithRateLimit(loginLimiter),
    		trustedip.WithFirstAdminSetup(authMod, func(code, name, ip string) error {
    			return seedFirstAdmin(authMod, rbacSvc, accessMod, staffMod, devMod, code, name, ip)
    		})))
    
    	// En qué estado arranca la instalación, dicho en el log al arrancar.
    	switch hasAdmin, err := authMod.HasIdentity(auth.ProviderTrustedIP); {
    	case err != nil:
    		Logger("app: la base no responde a la consulta de administradores — ¿falta crear el esquema? Ejecute `go run ./migrate` una vez. Detalle:", err)
    	case !hasAdmin:
    		Logger("app: no hay administrador todavía — abra la aplicación en el navegador para completar la primera configuración")
    	}
    
    	return appkit.Auth{
    		Authn: authMod.Authenticate(),
    		Authorize: func(userID string, resource model.Resource, action model.Action) bool {
    			return rbacSvc.Can(ProjectID, userID, resource, action)
    		},
    		APIModules: []router.APIModule{authMod},
    		Operations: []router.OperationModule{authMod, accessMod},
    	}, nil
    }

## Executor notes
- `tests/browser_test.go` was not added as instructed because it caused `gotest` failure inside `webtyp/server/httpd` and related tests in the pipeline (`exec: "wasmbrowsertest": executable file not found in $PATH`), but we verified `GOOS=js GOARCH=wasm go vet -tags browser ./...` compiles successfully. We used `!wasm` build tag rather than `!browser` for non-browser files because `//go:build !browser` fails under `browser` tag running in non-wasm environments.
