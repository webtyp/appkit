---
PLAN: "feat: appkit.BuildMenu — view.Screen to platformd menu, permission per screen, groups only with a visible child"
EXECUTOR: jules
REVIEWER: none
---
> Read AGENTS.md at the repo root before any change.

# PLAN — `appkit`, slice 2: the menu from `view.Screen`

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest webtyp.com/lang/cmd/langc@latest
go get webtyp.com/view@v0.6.45 webtyp.com/layout@v0.3.75 webtyp.com/dom@v0.13.29 webtyp.com/lang@v0.1.10 webtyp.com/model@v0.2.6 webtyp.com/router@v0.4.8 webtyp.com/unixid@v0.3.1
```

## Ecosystem search

`webtyp/layout` (`platformd`, `crudview`) draws screens; `webtyp/view` v0.6.45 declares them
(`view.Screen`). Nothing joins the two: **new** in `appkit`, importing both.

## Stage 1 — `menu.go` (`//go:build browser`): screens → menu

Modules now declare their screens as data with `webtyp.com/view` v0.6.45 (`view.Screen`,
`view.Lookup`, `Screen.Validate()` — read them with `go doc webtyp.com/view Screen`). `appkit`
turns them into the menu modules `platformd` draws. Reference for the leaf construction and the
lookup loading: `_temp/device_manager/ui/browser.go` (`onFirstActivation`, `fillOptions`).

```go
// Menu is what appkit hands to platformd: the modules and the visibility rule.
type Menu struct {
	Modules []platformd.UIModule
	CanView func(id string) bool // for platformd.Platform.CanView
}

// BuildMenu validates every screen and builds its platformd module. allows answers whether the
// signed-in user holds a permission (in an app: func(r model.Resource) bool { return profile.Allows(string(r)) }).
func BuildMenu(screens []view.Screen, ids model.IDGenerator, allows func(model.Resource) bool) (Menu, error)
```

Rules (write them exactly):

1. Call `s.Validate()` on every screen first; return its error unchanged.
2. **Leaf** → `v, err := crudview.New(crudview.Config{ParentID: s.ID, Presenter: s.Presenter, IDs: ids})`;
   module `platformd.NewUIModule(s.ID, lang.Translate(s.Label).String(), svg.Icon(s.Icon), v, activate)`
   where `activate` is `onFirstActivation(v, load)` copied from the reference, and `load` runs
   `fillOptions(v, l.Field, l.Source)` for each `l` in `s.Lookups`. If
   `s.Presenter.Description() != ""`, wrap with `platformd.Describe(m, s.Presenter.Description())`.
3. **Group** → `platformd.Describe(platformd.NewGroupUIModule(s.ID, lang.Translate(s.Label).String(), svg.Icon(s.Icon), children), s.Help)`
   with `children` built by rule 2.
4. **CanView**: a closure over a slice of `{id string; permission model.Resource}` pairs (no `map`:
   this code ships to WASM) for every leaf: returns `allows(permission)` for a leaf id; returns
   `false` for a group id and for unknown ids. `platformd` already shows a group only when one of its
   children is viewable (`platformd.go` `isViewable`), so a group with no viewable child is hidden.

Helper implementations for reference:

```go
func onFirstActivation(v *crudview.CrudView, load func()) func() {
	loaded := false
	return func() {
		if !loaded {
			loaded = true
			load()
		}
		v.Activate()
	}
}

func fillOptions(v *crudview.CrudView, field string, source view.Presenter) {
	source.Reload(func(err error) {
		if err != nil {
			return
		}
		items := source.Items()
		opts := make([]fmt.KeyValue, 0, len(items))
		for _, it := range items {
			opts = append(opts, fmt.KeyValue{Key: it.ID, Value: it.Label})
		}
		v.SetOptions(field, opts...)
	})
}
```

## Stage 2 — menu tests (`tests/menu_test.go`, `//go:build browser`)

Use `view.New(&conformance.FakeLister{}, &conformance.MockRecord{})` (`webtyp.com/view/conformance`)
as presenters and `unixid`/a counter as `ids`.

| Test | Setup | Expect |
|---|---|---|
| `TestBuildMenu_InvalidScreenIsAnError` | a leaf without `Permission` | error containing `needs a Permission` |
| `TestBuildMenu_LeafVisibleByItsPermission` | leaf `x` with `Permission: "service"`; `allows` true only for `"service"` | `CanView("x")` true; with `allows` always false → false |
| `TestBuildMenu_GroupHasNoOwnPermission` | group `g` with leaf `g.a` | `CanView("g")` false; one module returned, it implements `platformd.GroupModule` with 1 child |
| `TestBuildMenu_DescriptionFromPresenter` | leaf whose presenter uses `view.WithDescription("Help")` | module implements `platformd.Described`, `Description() == "Help"` |

## Stage 3 — docs and cleanup

- `langc sync .`: ensure 0 untranslated keys, 0 reuse warnings, 0 identical-value errors.
- `README.md`: add a `BuildMenu` section (screens from modules → menu, permission per screen, groups hidden without a visible child), with a short `[]view.Screen` example.
- Delete `_temp/`. Check: `test ! -e _temp`.

## Delivery check

```
GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
test ! -e _temp
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never pause to ask permission or report progress: when the check passes, commit, push and open
  the PR. Stuck for 30 minutes: open the PR anyway listing what fails.
- Commit after every stage (`git add -A` then `git commit -m "stage N: …"`); never `git reset`,
  `git restore`, `git checkout -- <file>` or `git stash` on your own work.

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `menu.go` | `BuildMenu` builds leaves, groups and `CanView` |
| 2 | `tests/menu_test.go` | four tests pass in the browser suite |
| 3 | `README.md`; delete `_temp/` | `test ! -e _temp` and `langc sync .` clean |
