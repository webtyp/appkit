# Tests

| Test | Rule |
|---|---|
| [TestBrowserBuild_AppkitImportsNoServerCode](../tests/browser_build_test.go) | la raíz de `appkit` y `appkit/browser` no importan código de servidor |
| [TestCallQueueAdapter_AllMethods](../tests/external_calls_test.go) | CallQueueAdapter delegates queue operations to externalcalls.Queue |
| [TestCallRunner_DeclarationAndRun](../tests/external_calls_test.go) | CallRunner resolves call declarations and executes external calls |
| [TestBuild_WiresQueueAndRunner](../tests/external_calls_test.go) | Build initializes the call queue and runner for modules with external calls |
| [TestCallExecutor_RecoversExpiredReplies](../tests/external_calls_test.go) | Call executor recovers expired WaitForReply calls to failed state |
| [TestFindBuiltModule_ReturnsTheModuleBuiltBefore](../tests/kit_test.go) | FindBuiltModule returns the module built earlier by its name |
| [TestFindBuiltModule_ProviderAfterConsumerNamesTheMissingOne](../tests/kit_test.go) | asking for a module built later is an error that names it |
| [TestBuild_ModulesAreBuiltInAppOrder](../tests/kit_test.go) | modules are built in App.Modules order, not in the order of the parts |
| [TestBuild_ModuleWithoutItsPartIsAnError](../tests/kit_test.go) | a listed module without a backend.Module is ErrNoPart |
| [TestBuild_PartOfAnUnlistedModuleIsAnError](../tests/kit_test.go) | a backend.Module of a module not in App.Modules is ErrUnlistedPart |
| [TestBuild_PartWithoutBuildIsAnError](../tests/kit_test.go) | a backend.Module without Build is ErrNoBuildFunc |
| [TestBuild_ModuleErrorIsNamed](../tests/kit_test.go) | a module's build error carries the module name |
| [TestBuild_NeitherCrudNorPartsIsAnError](../tests/kit_test.go) | a module that is neither crud.Module nor Parts is ErrNotCrudModule |
| [TestBuild_PartsAreFoundLikeModules](../tests/kit_test.go) | the parts of a module are found by FindBuiltModule by their name |
| [TestBuild_DepsCarryTheInstallation](../tests/kit_test.go) | Deps carries the zone, tenant, database, ids, events, queue and log |
| [TestBuild_WhatTheAppForgotIsNamed](../tests/kit_test.go) | a missing log, events, API, ids, driver, valid zone or sign-in is a named error |
| [TestBuild_WithoutAddressBuildsNothingAndDeniesAll](../tests/kit_test.go) | without a database address no module is built and everything is denied |
| [TestBuild_NoSignInSaysSo](../tests/kit_test.go) | without App.SignIn the log says nobody can enter and every operation is denied |
| [TestBuild_SignInGivesTheSessionAndTheRoles](../tests/kit_test.go) | a chosen sign-in mode gives Authn and an Authorize that refuses a person with no role |
| [TestBuild_ModesGetThePiecesAndMakeAdminGivesTheRole](../tests/kit_test.go) | each mode is built with every piece (identities, MakeAdmin, HasAdmin, Built); MakeAdmin makes HasAdmin true; Built finds the app's module |
| [TestBuild_AModeThatCannotBeBuiltStopsTheStart](../tests/kit_test.go) | a mode whose Build fails stops Build with that error |
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
| [TestNewSession_WhatTheAppForgotIsNamed](../tests/session_test.go) | a missing log, caller, ids, zone, screens part or unlisted part is a named error |
| [TestNewSession_WithoutSignInThereAreNoEntryScreens](../tests/session_test.go) | without App.SignIn there are no entry screens, no sign-out and no administration screens |
| [TestNewSession_NobodySignedInGetsTheOpenModesScreensAndTheAdministration](../tests/session_test.go) | nobody signed in: one entry screen per mode open at this door, SignOut, an idle lock, and the users and roles administration first |
| [TestNewSession_SignedInUserComesWithGrantsAndNoEntryScreens](../tests/session_test.go) | a signed-in user comes in Session.User with their grants; no entry screens |
| [TestNewSession_ServerDownIsNotTheEntryScreen](../tests/session_test.go) | when who-am-I gets no answer, Session.ServerDown and no entry screens (D76) |
| [TestMount_EachModeBringsItsOwnRoutes](../tests/kit_test.go) | with SignIn, Mount publishes each mode's own routes and the shared sign-out |
| [TestMount_EverySignedInPersonReadsMe](../tests/kit_test.go) | anyone signed in passes the permission of the crud read me (rbac); without a session it is refused |
| [TestMount_WithoutSignInThereAreNoSessionRoutes](../tests/kit_test.go) | Sin `SignIn` no hay rutas de sesión |
| [TestBuild_TrustedIPModeUsesTheAppsRegister](../tests/kit_test.go) | the real trusted_ip mode through appkit: the first administrator reaches the app's register with the setup IP, and each sign-in asks it about the caller's IP |
| [TestBuild_RejectsUnsupportedIndustry](../tests/configurable_test.go) | Build rejects a module that does not support App.Industry with ErrUnsupportedIndustry |
| [TestBuild_EnsuresAppRolesAndModuleUseGrants](../tests/configurable_test.go) | Build ensures App.Roles in access and grants permissions per ModuleUse in rbac |
| [TestBuild_RepeatedFailedSignInsRaiseOneAlert](../tests/security_watch_test.go) | ten failed sign-ins from one IP raise one notify alert (security.authn_login_fail_max); a forged token raises one at once |
