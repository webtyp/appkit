# Tests

| Test | Rule |
|---|---|
| [TestBrowserBuild_AppkitImportsNoServerCode](../tests/browser_build_test.go) | appkit's browser files import no server code (orm, mcp, rbac, httpd, unixid, rpc, inprocess) |
| [TestCallQueueAdapter_AllMethods](../tests/external_calls_test.go) | CallQueueAdapter delegates queue operations to externalcalls.Queue |
| [TestCallRunner_DeclarationAndRun](../tests/external_calls_test.go) | CallRunner resolves call declarations and executes external calls |
| [TestBuild_WiresQueueAndRunner](../tests/external_calls_test.go) | Build initializes the call queue and runner for modules with external calls |
| [TestCallExecutor_RecoversExpiredReplies](../tests/external_calls_test.go) | Call executor recovers expired WaitForReply calls to failed state |
| [TestFindBuiltModule_ReturnsTheModuleBuiltBefore](../tests/kit_test.go) | FindBuiltModule returns the module built earlier by its name |
| [TestFindBuiltModule_ProviderAfterConsumerNamesTheMissingOne](../tests/kit_test.go) | asking for a module built later is an error that names it |
| [TestBuild_ModulesAreBuiltInAppOrder](../tests/kit_test.go) | modules are built in App.Modules order, not in the order of the parts |
| [TestBuild_ModuleWithoutItsPartIsAnError](../tests/kit_test.go) | a listed module without a BackendModule is ErrNoPart |
| [TestBuild_PartOfAnUnlistedModuleIsAnError](../tests/kit_test.go) | a BackendModule of a module not in App.Modules is ErrUnlistedPart |
| [TestBuild_PartWithoutBuildIsAnError](../tests/kit_test.go) | a BackendModule without Build is ErrNoBuildFunc |
| [TestBuild_ModuleErrorIsNamed](../tests/kit_test.go) | a module's build error carries the module name |
| [TestBuild_NeitherCrudNorPartsIsAnError](../tests/kit_test.go) | a module that is neither crud.Module nor Parts is ErrNotCrudModule |
| [TestBuild_PartsAreFoundLikeModules](../tests/kit_test.go) | the parts of a module are found by FindBuiltModule by their name |
| [TestBuild_DepsCarryTheInstallation](../tests/kit_test.go) | Deps carries the zone, tenant, database, ids, events, queue and log |
| [TestBuild_WhatTheAppForgotIsNamed](../tests/kit_test.go) | a missing log, events, API, ids, driver, valid zone or sign-in is a named error |
| [TestBuild_WithoutAddressBuildsNothingAndDeniesAll](../tests/kit_test.go) | without a database address no module is built and everything is denied |
| [TestBuild_NoSignInSaysSo](../tests/kit_test.go) | without App.SignIn the log says nobody can enter and every operation is denied |
| [TestBuild_SignInByTrustedIPGivesTheSessionAndTheRoles](../tests/kit_test.go) | SignInByTrustedIP gives Authn and an Authorize that refuses a person with no role |
| [TestBuild_DefaultGrantsGoToTheAuthenticatedRole](../tests/kit_test.go) | DefaultGrants of the modules are ensured on the authenticated role with appkit's own rbac |
| [TestBuildModules_NeedsADatabase](../tests/kit_test.go) | BuildModules without a database is ErrNoDatabase |
| [TestBuildModules_BuildsOnTheGivenDatabaseInOrder](../tests/kit_test.go) | BuildModules builds on the given transaction, in order, and returns Deps that find them |
| [TestBuildModules_ErrorsAreNamed](../tests/kit_test.go) | BuildModules names the failing module and validates the zone |
| [TestApp_RenderHTMLNeedsAName](../tests/kit_test.go) | App.RenderHTML pre-renders the loading card only when the app has a name |
| [TestMount_PublishesTheOperationsThroughTheAPI](../tests/kit_test.go) | Mount publishes the operations through Backend.API |
| [TestMount_AnonymousIsDenied](../tests/kit_test.go) | an anonymous caller is refused with 403 |
| [TestMount_NoAgentsNoMCPEndpoint](../tests/kit_test.go) | without Backend.Agents no MCP endpoint is mounted |
| [TestMount_AgentsOpenOnlyTheAllowedTools](../tests/kit_test.go) | Agents expose only the operations in Allowed |
| [TestMount_AgentsWithNothingAllowedListNothing](../tests/kit_test.go) | Agents with nothing allowed list zero tools |
| [TestMount_PublicActionIsMountedPublic](../tests/kit_test.go) | a Public action is mounted with AccessPublic |
| [TestMount_PublicActionNoModuleOffersIsAnError](../tests/kit_test.go) | a Public action no module offers is rpc.ErrPublicNotMounted |
| [TestMount_TheAppsOwnRoutesAndExtraMountsAreMounted](../tests/kit_test.go) | Backend.Routes and the extra mounts are mounted |
| [TestMount_AnExtraMountErrorStopsTheMount](../tests/kit_test.go) | an error from an extra mount stops Mount |
| [TestLeftovers_ServerWarnsAtStart](../tests/leftovers_test.go) | Leftover columns in the database are logged at start and never dropped |
| [TestLeftovers_ServerStartsWhenUncheckable](../tests/leftovers_test.go) | A connection that cannot be checked logs one line and still starts |
| [TestLeftovers_NoSchemaNoCheck](../tests/leftovers_test.go) | An empty schema skips the leftovers check |
| [TestNewSession_CarriesWhatTheAppIs](../tests/session_test.go) | NewSession gives the layout the name, caller, zone, log and the screens in App.Modules order |
| [TestNewSession_ScreensGetZoneIdsAndCountries](../tests/session_test.go) | every module's screens get the zone, the ids and the caller in view.BrowserDeps |
| [TestNewSession_ScreensShareOnePageOfRecords](../tests/session_test.go) | every module gets the same records caller |
| [TestNewSession_WhatTheAppForgotIsNamed](../tests/session_test.go) | a missing log, caller, ids, zone, screens part, unlisted part or unknown sign-in is a named error |
| [TestNewSession_WithoutSignInThereAreNoEntryScreens](../tests/session_test.go) | without App.SignIn there are no entry screens and no administration screens |
| [TestNewSession_SignInByTrustedIPBringsTheEntryAndTheAdministration](../tests/session_test.go) | SignInByTrustedIP gives the login, the first setup and the users and roles administration first |
| [TestMount_SignInBringsItsOwnRoutes](../tests/kit_test.go) | Con `SignIn`, `Mount` publica el ingreso, la salida y `/me` |
| [TestMount_WithoutSignInThereAreNoSessionRoutes](../tests/kit_test.go) | Sin `SignIn` no hay rutas de sesión |
| [TestBuild_SignInUsesTheStaffRegister](../tests/kit_test.go) | El primer administrador llega al registro de personal con la IP de la configuración inicial, y cada ingreso le pregunta por la IP de quien llama |
