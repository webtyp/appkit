---
PLAN: "feat!: the installation's time zone reaches every module as a loaded time.Zone (server Deps.Zone, browser BrowserDeps)"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 5460934571777480108
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

# Plan — the clinic's zone, loaded once, handed to every module (breaking, no compatibility layer)

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/time@v0.6.3
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`, `go mod tidy` after `go get`.

## Why

`webtyp.com/time` v0.6.3 removed every implicit "local time": a local date or hour now comes only from a `time.Zone`
(`time.LoadZone(name)`). An app declares its installation's zone once (`App.Timezone`, e.g. `"America/Santiago"`),
but today it reaches modules as a bare string on the server (`Deps.Timezone`) and **not at all** in the browser
(`ModuleBrowser.Screens func(caller router.Caller)`), so a screen cannot show the clinic's hour. Owner decision
(webtyp/docs/TIME_ZONE_MASTER_PLAN.md, P1 = A): a dependency bag for the browser, like `Deps` on the server.

## Ecosystem search

- `Deps` (`module_server.go`) is the server's bag → **same pattern** for the browser: `BrowserDeps`.
- `time.LoadZone`, `time.Zone`, `time.ErrUnknownZone` (time v0.6.3 `zone.go`) → **reuse**.
- No other repo builds module screens → the change lives here; the 4 modules that set `ModuleBrowser.Screens`
  (`staff_manager`, `room_layout`, `device_manager`, `service_catalog`) adapt in their own plans.

## Design gate

1. Prior art: dependency injection bags (Go `http.Request.Context`, Spring/Angular injected config) — unverified;
   this ecosystem's own `appkit.Deps`. Followed.
2. Names: `BrowserDeps{Caller, Zone}`; `Deps.Zone` (replaces `Deps.Timezone`).
3. Ledger: +1 type; `Deps.Timezone string` → `Deps.Zone time.Zone` (one way: modules never call `LoadZone`
   themselves); `Screens` signature changes once, future needs (language, tenant) become fields, not new parameters.
4. Belongs in `appkit`: the composition root that already reads `App.Timezone`.
5. Deletes: `Deps.Timezone` (string). An app with an empty or unknown `App.Timezone` no longer starts.

## APIs used (webtyp.com/time v0.6.3)

| Symbol | Signature |
|---|---|
| `time.LoadZone` | `func LoadZone(name string) (Zone, error)` — `""` or unknown → `ErrUnknownZone` |
| `time.Zone` | value type; zero value panics when used |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — server: `Deps.Zone`

`module_server.go`: in `type Deps`, replace `Timezone string` with:

```go
	Zone time.Zone // the installation's zone (App.Timezone), loaded once: every local date and hour comes from it
```

New error in `migrate.go` next to `ErrNoOpen`, same constructor: `ErrNoTimezone = errors.New("appkit: App.Timezone is empty or not a known time zone — set an IANA name such as America/Santiago")`.

`serve.go` (`BuildServer`) and `build_modules.go` (`BuildModules`), right after `unixid.NewUnixID()`:

```go
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return nil, ErrNoTimezone // BuildModules: return Deps{}, ErrNoTimezone
	}
	d := Deps{ /* same fields */ Zone: zone } // instead of Timezone: app.Timezone
```

Import `webtyp.com/time` with the alias the file already uses for it, or `wtime` if the file imports stdlib `time`.

## Stage 2 — browser: `BrowserDeps`

`module_browser.go`:

```go
// BrowserDeps is what every module's screens get from appkit in the browser: how to call the server and the
// installation's zone (App.Timezone, loaded once). A new need of the screens becomes a field here, never a new
// parameter.
//
// Example: Screens: func(d appkit.BrowserDeps) []view.Screen { return ui.Screens(d.Caller, d.Zone) }
type BrowserDeps struct {
	Caller router.Caller
	Zone   time.Zone
}

type ModuleBrowser struct {
	Screens func(d BrowserDeps) []view.Screen
}
```

`run_browser.go`: `Screens(app App, caller router.Caller) []view.Screen` becomes
`Screens(app App, caller router.Caller) ([]view.Screen, error)`: it loads `time.LoadZone(app.Timezone)` (error →
`ErrNoTimezone`), builds `BrowserDeps{Caller: caller, Zone: zone}` and calls `m.Browser.Screens(d)`.
`accessui.Screens(caller)` stays as is. In `buildSession`, when `Screens` returns an error, the session shows that
error's text instead of the platform (use the existing `loginMessage`-style text element; never a blank page), and the
browser console gets it once. Move `ErrNoTimezone` to a file without a build tag if `migrate.go` is server-only, so both
binaries see it.

## Stage 3 — tests

| Test | Setup → call | Expected |
|---|---|---|
| `TestBuildServer_NoTimezone` | app without `Timezone` | `errors.Is(err, appkit.ErrNoTimezone)` |
| `TestBuildModules_BadTimezone` | `Timezone: "Mars/Base"` | `ErrNoTimezone` |
| `TestBuildModules_ZoneInDeps` | `Timezone: "America/Santiago"`, a module whose Build records `d.Zone.Name()` | `"America/Santiago"` |
| `TestScreens_GetZone` (browser build or a plain test of `Screens`) | module whose `Screens` records `d.Zone.Name()` | `"America/Santiago"` |
| existing tests | add `Timezone: "America/Santiago"` to every `appkit.App{…}` they build | pass |

## Stage 4 — docs

`README.md`: the `App.Timezone` field is required (IANA name) and reaches modules as `Deps.Zone` / `BrowserDeps.Zone`,
with the example above. `docs/ARCHITECTURE.md`: decision "the zone is loaded once by appkit; modules never load it".

## Delivery check

1. `gotest` passes (a PR that does not compile, or whose tests could not run, is a failed delivery).
2. `grep -rn "Timezone:" --include=*.go . | grep -v "_test.go\|app.go"` → empty (no `Deps.Timezone` left);
   `grep -n "func(caller router.Caller)" module_browser.go` → empty.

Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own
work. Stuck 30 minutes: open the PR anyway listing what fails.

## Stages

| Stage | Files | Done when |
|---|---|---|
| 1 | `module_server.go`, `serve.go`, `build_modules.go`, errors | compiles |
| 2 | `module_browser.go`, `run_browser.go` | compiles for `GOOS=js GOARCH=wasm -tags browser` (gotest checks) |
| 3 | `tests/*` | tests pass |
| 4 | `README.md`, `docs/ARCHITECTURE.md` | done |
