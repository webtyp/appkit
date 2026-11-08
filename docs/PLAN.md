---
PLAN: "refactor!: appkit drops layout — browser contracts come from view; the chassis bootstrap moves to layout/app"
EXECUTOR: antigravity
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit after every stage and stop. Never ask whether to
> proceed. This plan runs **locally** and is **published as soon as the delivery check passes** (no `replace` anywhere:
> every dependency is a published version). Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.

# Plan: `appkit` without `layout`

## Tools and local dependencies

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```
```bash
go get webtyp.com/view@latest   # must contain view.Demo (plan L1, published first); else stop
```
No `replace` in `go.mod`; `go mod tidy`.

## Why

Every module imports `appkit` (for `appkit.Module`, `appkit.BrowserDeps`), and `appkit/browser.go:12-13` imports
`webtyp.com/layout/app` and `webtyp.com/layout/platformd`. Go therefore writes `layout` into the `go.mod` of modules that
use nothing of it (`patient_directory`, `staff_manager`, `service_catalog`, `device_manager`, `business_calendar`:
`// indirect`), and every `layout` release forces them to re-publish. Decisions D16, D17 of
`webtyp/docs/ARCHITECTURE_MASTER_PLAN.md`: the browser contracts live in `view` (plan L1); the chassis bootstrap
(`Browser`) moves to `layout/app` (plan L3), which will import `appkit` — so `appkit` must not import `layout`.

## Design gate

1. **Prior art:** same as plan L1 (`view/docs/PLAN.md`): the piece declares, the host draws.
2. **Names:** `appkit.Entry` — «the entry screens of this app»; `appkit.Screens` unchanged.
3. **Ledger:** concepts −3 (`appkit.BrowserDeps`, `ModuleBrowser`, `EntryScreens` → `view`); ways to do the same thing ±0.
4. **Where:** `appkit` composes an `App` from modules; drawing the page belongs to the chassis (`layout`).
5. **Deletes:** `browser.go`, three types of `module_browser.go`, `tests/browser_test.go` (its cases move here and to L3).

## APIs used

| Symbol | Signature | From |
|---|---|---|
| `view.BrowserDeps` | `struct{ Caller router.Caller; Zone time.Zone }` | plan L1 |
| `view.ModuleBrowser` | `struct{ Screens func(d BrowserDeps) []Screen; Entry func(d BrowserDeps) EntryScreens }` | plan L1 |
| `view.EntryScreens` | `struct{ Login, Setup Entry; IdleSeconds int }` | plan L1 |
| `view.Entry` | `struct{ Subtitle lang.Text; Form dom.Component; Message *dom.SignalString }` | plan L1 |

If a signature differs, stop and name the symbol; never adapt it by guessing.

## Explicit deletions (grep must be empty)

- `browser.go` → `test ! -e browser.go`
- `type BrowserDeps`, `type ModuleBrowser`, `type EntryScreens` in `module_browser.go` → `grep -rn "type BrowserDeps\|type ModuleBrowser\|type EntryScreens" .`
- every import of layout → `grep -rn "webtyp.com/layout" .` (also absent from `go.mod` after `go mod tidy`)
- `tests/browser_test.go` → `test ! -e tests/browser_test.go`

## Stage 1 — `app.go`, `module_browser.go`

- `Module.Browser` becomes `view.ModuleBrowser`. Comment of `App.Timezone`: `Deps.Zone / view.BrowserDeps.Zone`.
- `module_browser.go` keeps only what is not moved; if nothing remains, delete the file.

## Stage 2 — `screens.go` (new, no build tag; replaces the two functions of `browser.go` that are not chassis)

```go
var ErrTwoEntryModules = errors.New("appkit: two modules set ModuleBrowser.Entry — only the sign-in module (webtyp/access) may set it; remove Entry from the other one")

// Screens are the menu's screens: each module's in App.Modules order (the first one opens first).
//
//	screens, err := appkit.Screens(config.App, caller)
func Screens(app App, caller router.Caller) ([]view.Screen, error) {
	d, err := browserDeps(app, caller)
	if err != nil {
		return nil, err
	}
	var out []view.Screen
	for _, m := range app.Modules {
		if m.Browser.Screens != nil {
			out = append(out, m.Browser.Screens(d)...)
		}
	}
	return out, nil
}

// Entry is the sign-in and first-setup screens of the app's entry module; zero when no module sets Entry.
//
//	entry, err := appkit.Entry(config.App, caller)
func Entry(app App, caller router.Caller) (view.EntryScreens, error) {
	d, err := browserDeps(app, caller)
	if err != nil {
		return view.EntryScreens{}, err
	}
	var out view.EntryScreens
	found := false
	for _, m := range app.Modules {
		if m.Browser.Entry == nil {
			continue
		}
		if found {
			return view.EntryScreens{}, ErrTwoEntryModules
		}
		out = m.Browser.Entry(d)
		found = true
	}
	return out, nil
}

func browserDeps(app App, caller router.Caller) (view.BrowserDeps, error) {
	zone, err := time.LoadZone(app.Timezone)
	if err != nil {
		return view.BrowserDeps{}, ErrNoTimezone
	}
	return view.BrowserDeps{Caller: caller, Zone: zone}, nil
}
```

Use the same error style as `module_server.go:18` (`ErrNoTimezone`). The old `panic("appkit: two entry modules…")`
becomes `ErrTwoEntryModules` (zero-debt: a returned error, never a panic in a library).

## Stage 3 — tests (`tests/screens_test.go`, rows in `docs/TESTS.md`)

| Test | Setup → call → expected |
|---|---|
| `TestScreens_GetZone` | moved from `tests/browser_test.go`: a module whose `Screens` records `d.Zone.Name()`; `Screens(app{Timezone:"America/Santiago"}, nil)` → recorded `"America/Santiago"` |
| `TestScreens_BadTimezone` | `Timezone: "Mars/Base"` → `ErrNoTimezone` |
| `TestEntry_NoneIsZero` | no module sets `Entry` → zero `view.EntryScreens`, `nil` |
| `TestEntry_OneModule` | one module returns `view.EntryScreens{IdleSeconds: 60}` → `IdleSeconds == 60` |
| `TestEntry_TwoModulesIsError` | two modules set `Entry` → `ErrTwoEntryModules` |

## Stage 4 — documentation

`README.md` and `docs/ARCHITECTURE.md`: every `appkit.Browser`, `appkit.BrowserDeps`, `appkit.ModuleBrowser`,
`appkit.EntryScreens` → `layoutapp.Browser` (from `webtyp.com/layout/app`, plan L3) and `view.*`; state in one line that
`appkit` never imports `layout`. `langc sync .` (zero pending).

## Delivery check

```bash
gotest
langc sync .
grep -rn "webtyp.com/layout" .    # empty, go.mod included
```

A PR that does not compile, or whose tests could not run, is a failed delivery.

| Stage | Files |
|---|---|
| 1 | `app.go`, `module_browser.go` |
| 2 | `screens.go`, delete `browser.go` |
| 3 | `tests/screens_test.go`, delete `tests/browser_test.go`, `docs/TESTS.md` |
| 4 | `README.md`, `docs/ARCHITECTURE.md`, `lang.json` |
