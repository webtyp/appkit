# Tests

| Test | Rule |
|---|---|
| [TestFind_OneProvider](../tests/app_test.go) | Find retrieves dependency provided by prior module |
| [TestFind_NoProvider](../tests/app_test.go) | Find returns ErrNoProvider when requested interface is missing |
| [TestFind_ProviderAfterConsumer](../tests/app_test.go) | Find fails if provider is registered after consumer |
| [TestFind_TwoProviders](../tests/app_test.go) | Find returns ErrManyProviders on ambiguity |
| [TestBuildServer_NoDSN](../tests/app_test.go) | BuildServer succeeds without database when DSN is empty |
| [TestBuildServer_DSNWithoutOpen](../tests/app_test.go) | BuildServer returns ErrNoOpen if DSN given but Database opener is nil |
| [TestBuildServer_NoTimezone](../tests/app_test.go) | BuildServer returns ErrNoTimezone if Timezone is invalid |
| [TestApp_NoLogIsAnError](../tests/app_test.go) | BuildServer returns ErrNoLog if Log is nil |
| [TestBrowserBuild_LeavesServerCodeOut](../tests/browser_build_test.go) | Browser build tags exclude server-only packages (orm, mcp, rbac, httpd) |
| [TestBuildModules_NilDB](../tests/build_modules_test.go) | BuildModules returns ErrNoDatabase when DB is nil |
| [TestBuildModules_FindsAppModules](../tests/build_modules_test.go) | BuildModules populates Deps so modules can Find each other |
| [TestBuildModules_PartsAddsAllParts](../tests/build_modules_test.go) | BuildModules expands Parts so parts are discoverable via Find |
| [TestBuildModules_ModuleErrorNamed](../tests/build_modules_test.go) | Module initialization errors carry module name |
| [TestBuildModules_BadTimezone](../tests/build_modules_test.go) | BuildModules validates Timezone |
| [TestBuildModules_ZoneInDeps](../tests/build_modules_test.go) | Deps receives loaded Timezone |
| [TestBuildModules_APIModuleIsNotMounted](../tests/infra_test.go) | Standalone API modules without CRUD or Parts are not registered as ops |
| [TestBuildModules_NeitherCrudNorPartsIsError](../tests/infra_test.go) | BuildModules returns ErrNotCrudModule if module produces neither CRUD nor Parts |
| [TestInfra_Mounted](../tests/infra_test.go) | AppServer.Infra modules are built and mounted on Router |
| [TestPublic_CreateMounted](../tests/infra_test.go) | Public actions declared on AppServer.Public are registered with AccessPublic |
| [TestPublic_UnknownIsError](../tests/infra_test.go) | Public declaration for unoffered entity returns ErrPublicNotMounted |
| [TestGrants_AppliedAtBoot](../tests/infra_test.go) | ModuleServer.DefaultGrants are merged and passed to AppServer.Grants |
| [TestGrants_MissingGranterIsError](../tests/infra_test.go) | DefaultGrants without AppServer.Grants returns ErrNoGrants |
| [TestLeftovers_ServerWarnsAtStart](../tests/leftovers_test.go) | Leftover columns in DB trigger logger warnings |
| [TestLeftovers_ServerStartsWhenUncheckable](../tests/leftovers_test.go) | DB connection lacking DDL compiler logs warning without failing startup |
| [TestLeftovers_NoSchemaNoCheck](../tests/leftovers_test.go) | Empty schema skips leftovers check |
| [TestBuildServer_WithEntryModule](../tests/login_test.go) | Server integrates Entry module for authn, authz, and policy |
| [TestBuildServer_WithoutEntryModule](../tests/login_test.go) | Missing Entry module falls back to default deny with log warning |
| [TestBrand_RenderHTML](../tests/login_test.go) | App.RenderHTML pre-renders loading card before WASM loads |
| [TestScreens_GetZone](../tests/screens_test.go) | Screens receives configured time.Zone in BrowserDeps |
| [TestScreens_BadTimezone](../tests/screens_test.go) | Screens returns ErrNoTimezone on invalid zone |
| [TestEntryScreens_NoneIsZero](../tests/screens_test.go) | EntryScreens returns zero value when no module provides entry screens |
| [TestEntryScreens_OneModule](../tests/screens_test.go) | EntryScreens returns entry screens from module |
| [TestEntryScreens_TwoModulesIsError](../tests/screens_test.go) | Multiple modules declaring Entry returns ErrTwoEntryModules |
| [TestMCPClosedByDefault](../tests/server_test.go) | MCP endpoint lists zero tools when MCPTools is empty |
| [TestMCPOpensExposedTools](../tests/server_test.go) | MCP endpoint exposes only operations listed in MCPTools |
| [TestRPCMounted](../tests/server_test.go) | CRUD module operations mounted at /api/<module>/<entity> |
| [TestAnonymousDenied](../tests/server_test.go) | Non-public RPC routes reject unauthenticated requests with 403 |
| [TestDefaultDenyWithoutAuthorize](../tests/server_test.go) | Missing Authorize defaults to DenyAll |
| [TestRegisterMountsAppRoutes](../tests/server_test.go) | App-specific routes in Register are mounted |
| [TestMountsReceiveRouterAndPolicy](../tests/server_test.go) | Mounts hooks receive router and policy |
| [TestMountErrorStopsServer](../tests/server_test.go) | Error returned by Mounts hook halts MountServer |
| [TestNoDevToolsByDefault](../tests/server_test.go) | Dev tool routes not exposed in production build |
| [TestMountServer_PassesTheBrokerToRPC](../tests/server_test.go) | MountServer passes broker to rpc, which announces published writes |
| [TestMountServer_PassesTheBrokerToMCP](../tests/server_test.go) | MountServer passes broker to mcp, which announces executed tool writes |
| [TestMountServer_NilBrokerIsFine](../tests/server_test.go) | MountServer with nil Events performs writes without error |
| [TestMountServer_RestEndToEndOverHTTP](../tests/server_test.go) | Full CRUD lifecycle over real HTTP using rpc.Caller |
| [TestCheckPublic_FindsTheRouteByMethodAndPath](../tests/server_test.go) | checkPublic matches public route by method and path suffix |
| [TestCheckPublic_AWrongMethodIsNotAMatch](../tests/server_test.go) | checkPublic rejects mismatched route method |
| [TestApp_BrandIsViewBrand](../tests/session_test.go) | App.Brand is view.Brand |
| [TestLayout_ShowReceivesTheSession](../tests/session_test.go) | view.Layout.Show receives constructed Session |
| [TestNewSession_CarriesWhatTheAppIs](../tests/session_test.go) | NewSession compiles view.Session with brand, caller, zone, screens, entry, log |
| [TestNewSession_NoTimezoneIsAnError](../tests/session_test.go) | NewSession returns ErrNoTimezone when Timezone is omitted |
| [TestNewSession_ScreensShareOnePageOfRecords](../tests/session_test.go) | Screens receive common records caller instance |
| [TestNewSession_TwoEntryModulesIsAnError](../tests/session_test.go) | NewSession rejects multiple modules with Entry |
