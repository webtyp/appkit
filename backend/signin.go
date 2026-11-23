package backend

import (
	"webtyp.com/access"
	"webtyp.com/auth"
	"webtyp.com/auth/authority"
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/rbac"
	"webtyp.com/router"
	"webtyp.com/time"
)

const (
	roleAdmin          = model.RoleCode("admin")
	permWildcardAll    = "wildcard:all"
	defaultIdleMinutes = 30
	modeDevAutologin   = "dev_autologin" // the development auto-login is not news
)

// signIn is what appkit builds for App.SignIn: who the caller is, what they may do, the users and the roles. It
// builds every mode the app chose with the same pieces and never knows which mode it is.
type signIn struct {
	users  *authority.Module
	access *access.Module
	authn  router.Middleware
	authz  model.Authorizer
}

// parts are the crud modules the sign-in adds to the operations: users and roles with their administration.
func (s *signIn) parts() []model.ModuleNaming { return []model.ModuleNaming{s.users, s.access} }

// makeAdmin gives userID the administrator role, which holds every permission. Any mode that creates the first
// administrator calls it through auth.Ports.MakeAdmin.
func makeAdmin(d Deps, acc *access.Module, roles *rbac.Service, userID string) error {
	if err := acc.EnsureRoleFor(roleAdmin, "Admin", "System administrator", []string{userID}); err != nil {
		return err
	}
	role, err := roles.GetRoleByCode(d.TenantID, roleAdmin)
	if err != nil {
		return err
	}
	if err := roles.CreatePermission(d.TenantID, permWildcardAll, permWildcardAll, model.Wildcard, model.AllActions); err != nil {
		return err
	}
	return roles.AssignPermission(d.TenantID, role.Id, permWildcardAll)
}

// hasAdmin reports whether anyone holds the administrator role; no role yet means no administrator.
func hasAdmin(d Deps, roles *rbac.Service) (bool, error) {
	n, err := roles.RoleUserCount(d.TenantID, roleAdmin)
	if rbac.IsRoleNotFound(err) {
		return false, nil
	}
	return n > 0, err
}

// buildSignIn builds users, roles and every mode of App.SignIn on d.DB, once every module is built so a mode can find
// the module it needs (Ports.Built).
func buildSignIn(d Deps, modes []auth.Mode, broker events.Broker, rbacSvc *rbac.Service, idleMinutes int) (*signIn, error) {
	if broker != nil {
		if err := watchSecurity(broker, d.Log); err != nil {
			return nil, err
		}
	}
	if idleMinutes <= 0 {
		idleMinutes = defaultIdleMinutes
	}
	users, err := authority.New(d.DB, auth.Config{
		IDs:         d.IDs,
		Events:      d.Events,
		IdleTTL:     time.Unix(int64(idleMinutes)) * time.Minute,
		Permissions: &auth.Permissions{Grants: rbacSvc, ProjectID: d.TenantID},
	})
	if err != nil {
		return nil, err
	}
	users.SetLog(d.Log)
	acc, err := access.New(access.Deps{
		DB: d.DB, IDs: d.IDs, Users: users, Roles: rbacSvc, ProjectID: d.TenantID,
		Assurance: model.AssuranceSingleFactor,
	})
	if err != nil {
		return nil, err
	}

	ports := users.Ports()
	ports.MakeAdmin = func(userID string) error { return makeAdmin(d, acc, rbacSvc, userID) }
	ports.HasAdmin = func() (bool, error) { return hasAdmin(d, rbacSvc) }
	ports.Built = func(def model.ModuleDefinition) (model.ModuleNaming, error) { return FindBuiltModule(d, def.Name) }
	ports.Limiter = auth.NewIPLimiter(maxLoginAttempts, loginWindow, loginBlock)
	for _, mode := range modes {
		a, err := mode.Build(ports)
		if err != nil {
			return nil, err
		}
		users.Enable(a)
		if giver, ok := a.(auth.GivesSignIn); ok {
			if late, ok := d.SignInAccounts.(*lateAccounts); ok && late.giver == nil {
				late.giver = giver
			}
		}
	}

	switch exists, err := ports.HasAdmin(); {
	case err != nil:
		d.Log(msgNoAdminQuery, err)
	case !exists:
		d.Log(msgNoAdmin)
	}

	return &signIn{
		users:  users,
		access: acc,
		authn:  users.Authenticate(),
		authz: func(userID string, resource model.Resource, action model.Action) bool {
			return rbacSvc.Can(d.TenantID, userID, resource, action)
		},
	}, nil
}

const (
	msgNoAdminQuery = "appkit: the database does not answer the administrators query — run go run ./migrate once. Detail:"
	msgNoAdmin      = "appkit: no administrator yet — open the app in the browser to complete the first setup"
)
