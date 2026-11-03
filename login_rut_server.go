//go:build !wasm

package appkit

import (
	"errors"
	"fmt"
	"webtyp.com/time"

	"webtyp.com/access"
	"webtyp.com/auth"
	"webtyp.com/auth/authority"
	trustedip "webtyp.com/auth/trusted_ip"
	"webtyp.com/env"
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/rbac"
	"webtyp.com/router"
)

const (
	EnvSessionIdleMinutes     = "SESSION_IDLE_MINUTES"
	defaultSessionIdleMinutes = 30
	maxLoginAttempts          = 3
	loginWindow               = 15 * 60 // seconds
	loginBlock                = 15 * 60
	RoleAdmin                 = model.RoleCode("admin")
	permWildcardAll           = "wildcard:all"
)

type serverAuth struct {
	Authn      router.Middleware
	Authorize  model.Authorizer // nil = deny all
	Policy     model.PolicyDescriber
	APIModules []router.APIModule       // auth's own endpoints (login, me, logout)
	Operations []router.OperationModule // auth's own rpc operations (users, roles), published with the modules'
}

type eventBus interface {
	Subscribe(topic string, h events.Handler) events.Subscription
}

func securityEventLogger(logger func(...any)) func(auth.SecurityEvent) {
	return func(e auth.SecurityEvent) {
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

func (l LoginByRUT) build(d Deps, logger func(...any)) (serverAuth, error) {
	if bus, ok := d.Events.(eventBus); ok {
		logSecurityEvent := securityEventLogger(logger)
		bus.Subscribe(auth.TopicSecurity, func(e events.Event) {
			if se, ok := e.Payload.(*auth.SecurityEvent); ok {
				logSecurityEvent(*se)
			}
		})
	}

	rbacSvc, err := rbac.New(d.DB)
	if err != nil {
		return serverAuth{}, err
	}

	idleMinutes := defaultSessionIdleMinutes
	if l.IdleMinutes > 0 {
		idleMinutes = l.IdleMinutes
	} else if v := env.Get(EnvSessionIdleMinutes); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			idleMinutes = n
		} else {
			if logger != nil {
				logger("app: Warning: invalid " + EnvSessionIdleMinutes + ", using default 30")
			}
		}
	}

	authMod, err := authority.New(d.DB, auth.Config{
		IDs:         d.IDs,
		Events:      d.Events,
		IdleTTL:     time.Unix(int64(idleMinutes)) * time.Minute,
		Permissions: &auth.Permissions{Grants: rbacSvc, ProjectID: d.TenantID},
	})
	if err != nil {
		return serverAuth{}, err
	}
	authMod.SetLog(logger)

	accessMod, err := access.New(access.Deps{
		DB: d.DB, IDs: d.IDs, Users: authMod, Roles: rbacSvc, ProjectID: d.TenantID,
		// La administración de roles se hace en la clínica, entrando con RUT (un factor).
		Assurance: model.AssuranceSingleFactor,
	})
	if err != nil {
		return serverAuth{}, err
	}

	trustedStore, err := Find[auth.TrustedIPStore](d)
	if err != nil {
		return serverAuth{}, err
	}

	loginLimiter := auth.NewIPLimiter(maxLoginAttempts, loginWindow, loginBlock)
	authMod.Enable(trustedip.New(authMod, trustedStore, authMod, authMod, false,
		trustedip.WithRateLimit(loginLimiter),
		trustedip.WithFirstAdminSetup(authMod, func(code, name, ip string) error {
			return seedFirstAdmin(d, authMod, rbacSvc, accessMod, code, name, ip)
		})))

	if logger != nil {
		switch hasAdmin, err := authMod.HasIdentity(auth.ProviderTrustedIP); {
		case err != nil:
			logger("appkit: the database does not answer the administrators query — run go run ./migrate once. Detail:", err)
		case !hasAdmin:
			logger("appkit: no administrator yet — open the app in the browser to complete the first setup")
		}
	}

	return serverAuth{
		Authn: authMod.Authenticate(),
		Authorize: func(userID string, resource model.Resource, action model.Action) bool {
			return rbacSvc.Can(d.TenantID, userID, resource, action)
		},
		APIModules: []router.APIModule{authMod},
		Operations: []router.OperationModule{authMod, accessMod},
	}, nil
}
