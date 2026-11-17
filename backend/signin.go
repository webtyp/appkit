package backend

import (
	"webtyp.com/access"
	"webtyp.com/auth"
	"webtyp.com/auth/authority"
	trustedip "webtyp.com/auth/trusted_ip"
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/rbac"
	"webtyp.com/router"
	"webtyp.com/time"
)

const (
	maxLoginAttempts   = 3
	loginWindow        = 15 * 60 // seconds
	loginBlock         = 15 * 60
	roleAdmin          = model.RoleCode("admin")
	permWildcardAll    = "wildcard:all"
	defaultIdleMinutes = 30
)

// staffDevices is what a built module offers when it is the staff register: from which IP a person may enter and how
// the first administrator gets its device and staff record. Optional: with none built, any IP is refused and only the
// user and its role are created. appkit asserts it on the built modules; it is not exported.
type staffDevices interface {
	auth.TrustedIPStore
	RegisterFirstAdmin(userID, code, name, ip string) error
}

// findStaffDevices is the one built module that is the staff register, or nil.
func findStaffDevices(d Deps) staffDevices {
	if d.built == nil {
		return nil
	}
	for _, m := range *d.built {
		if s, ok := m.v.(staffDevices); ok {
			return s
		}
	}
	return nil
}

// signIn is what appkit builds for App.SignIn: who the caller is, what they may do, the users and the roles.
type signIn struct {
	users  *authority.Module
	access *access.Module
	rbac   *rbac.Service
	authn  router.Middleware
	authz  model.Authorizer
}

// parts are the crud modules the sign-in adds to the operations: users and roles with their administration.
func (s *signIn) parts() []model.ModuleNaming { return []model.ModuleNaming{s.users, s.access} }

func securityEventLogger(logger func(...any)) func(auth.SecurityEvent) {
	return func(e auth.SecurityEvent) {
		if e.Type == auth.EventDevAutologin {
			return
		}
		args := []any{"app: SECURITY EVENT — type:", e.Type}
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

func seedFirstAdmin(d Deps, s *signIn, code, name, ip string) error {
	user, err := s.users.CreateUser("", name, "")
	if err != nil {
		return err
	}
	if err := s.users.RegisterLAN(user.Id, code); err != nil {
		return err
	}
	if staff := findStaffDevices(d); staff != nil {
		if err := staff.RegisterFirstAdmin(user.Id, code, name, ip); err != nil {
			return err
		}
	}
	if err := s.access.EnsureRoleFor(roleAdmin, "Admin", "System administrator", []string{user.Id}); err != nil {
		return err
	}
	role, err := s.rbac.GetRoleByCode(d.TenantID, roleAdmin)
	if err != nil {
		return err
	}
	if err := s.rbac.CreatePermission(d.TenantID, permWildcardAll, permWildcardAll, model.Wildcard, model.AllActions); err != nil {
		return err
	}
	return s.rbac.AssignPermission(d.TenantID, role.Id, permWildcardAll)
}

// buildSignIn builds the sign-in by trusted IP on d.DB, once every module is built so the staff register can be found.
func buildSignIn(d Deps, broker events.Subscriber, rbacSvc *rbac.Service, idleMinutes int) (*signIn, error) {
	logger := d.Log
	if broker != nil {
		logSecurityEvent := securityEventLogger(logger)
		broker.Subscribe(auth.TopicSecurity, func(e events.Event) {
			if se, ok := e.Payload.(*auth.SecurityEvent); ok {
				logSecurityEvent(*se)
			}
		})
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
	users.SetLog(logger)
	acc, err := access.New(access.Deps{
		DB: d.DB, IDs: d.IDs, Users: users, Roles: rbacSvc, ProjectID: d.TenantID,
		Assurance: model.AssuranceSingleFactor,
	})
	if err != nil {
		return nil, err
	}
	s := &signIn{users: users, access: acc, rbac: rbacSvc}

	var trusted auth.TrustedIPStore = refuseAllIPs{}
	if staff := findStaffDevices(d); staff != nil {
		trusted = staff
	}
	limiter := auth.NewIPLimiter(maxLoginAttempts, loginWindow, loginBlock)
	users.Enable(trustedip.New(users, trusted, users, users, false,
		trustedip.WithRateLimit(limiter),
		trustedip.WithFirstAdminSetup(users, func(code, name, ip string) error {
			return seedFirstAdmin(d, s, code, name, ip)
		})))

	switch hasAdmin, err := users.HasIdentity(auth.ProviderTrustedIP); {
	case err != nil:
		logger(msgNoAdminQuery, err)
	case !hasAdmin:
		logger(msgNoAdmin)
	}

	s.authn = users.Authenticate()
	s.authz = func(userID string, resource model.Resource, action model.Action) bool {
		return rbacSvc.Can(d.TenantID, userID, resource, action)
	}
	return s, nil
}

const (
	msgNoAdminQuery = "appkit: the database does not answer the administrators query — run go run ./migrate once. Detail:"
	msgNoAdmin      = "appkit: no administrator yet — open the app in the browser to complete the first setup"
)

// refuseAllIPs is the trusted-IP register when no staff module is built: nobody enters from anywhere.
type refuseAllIPs struct{}

func (refuseAllIPs) IsTrustedIP(string, string) bool { return false }
