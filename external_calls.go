//go:build !browser

package appkit

import (
	"sync"

	"webtyp.com/crud"
	"webtyp.com/externalcalls"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/time"
)

var (
	executorMu   sync.Mutex
	executorStop func()
)

// CallQueueAdapter adapts an *externalcalls.Queue to satisfy crud.CallQueue.
type CallQueueAdapter struct {
	q *externalcalls.Queue
}

// NewCallQueue creates a crud.CallQueue backed by q.
//
// Example: queue := appkit.NewCallQueue(q)
func NewCallQueue(q *externalcalls.Queue) crud.CallQueue {
	if q == nil {
		return nil
	}
	return &CallQueueAdapter{q: q}
}

// Add delegates to the underlying externalcalls.Queue.Add.
func (a *CallQueueAdapter) Add(tx crud.Tx, r crud.CallRequest) (string, error) {
	return a.q.Add(tx, externalcalls.Request{
		TenantId: r.TenantId,
		Module:   r.Module,
		Entity:   r.Entity,
		RecordId: r.RecordId,
		Call:     r.Call,
		Mode:     r.Mode,
	})
}

// MarkDone delegates to the underlying externalcalls.Queue.MarkDone.
func (a *CallQueueAdapter) MarkDone(id string) error {
	return a.q.MarkDone(id)
}

// MarkFailedAttempt delegates to the underlying externalcalls.Queue.MarkFailedAttempt.
func (a *CallQueueAdapter) MarkFailedAttempt(id string, call model.ExternalCallDefinition, reason string) error {
	return a.q.MarkFailedAttempt(id, call, reason)
}

// ConfirmCall delegates to the underlying externalcalls.Queue.ConfirmCall.
func (a *CallQueueAdapter) ConfirmCall(id string) error {
	return a.q.ConfirmCall(id)
}

// RejectCall delegates to the underlying externalcalls.Queue.RejectCall.
func (a *CallQueueAdapter) RejectCall(id, reason string) error {
	return a.q.RejectCall(id, reason)
}

type callTarget struct {
	declaration model.ExternalCallDefinition
	store       crud.Store
	newList     func() model.ModelSlice
	fn          crud.ExternalCallFunc
}

// CallRunner implements externalcalls.Runner by inspecting the entities and calls declared across modules.
type CallRunner struct {
	targets map[string]map[string]map[string]callTarget // module -> entity -> call
}

// NewCallRunner constructs an externalcalls.Runner from the provided crud modules.
//
// Example: runner := appkit.NewCallRunner(ops)
func NewCallRunner(ops []crud.Module) externalcalls.Runner {
	r := &CallRunner{
		targets: make(map[string]map[string]map[string]callTarget),
	}
	for _, m := range ops {
		if m == nil {
			continue
		}
		ci := &callInspector{}
		m.MountOperations(ci)
		modName := m.ModelName()
		for _, e := range ci.entities {
			entityName := e.Model.ModelName()
			for _, def := range e.ExternalCalls {
				fn := e.Calls[def.Name]
				if r.targets[modName] == nil {
					r.targets[modName] = make(map[string]map[string]callTarget)
				}
				if r.targets[modName][entityName] == nil {
					r.targets[modName][entityName] = make(map[string]callTarget)
				}
				r.targets[modName][entityName][def.Name] = callTarget{
					declaration: def,
					store:       e.Store,
					newList:     e.NewList,
					fn:          fn,
				}
			}
		}
	}
	return r
}

// Declaration answers the declaration of call on entity in module.
func (r *CallRunner) Declaration(module, entity, call string) (model.ExternalCallDefinition, bool) {
	if r.targets[module] == nil || r.targets[module][entity] == nil {
		return model.ExternalCallDefinition{}, false
	}
	target, ok := r.targets[module][entity][call]
	if !ok {
		return model.ExternalCallDefinition{}, false
	}
	return target.declaration, true
}

// Run executes the queued external call for its record.
func (r *CallRunner) Run(call externalcalls.QueuedCall, key string) error {
	if r.targets[call.Module] == nil || r.targets[call.Module][call.Entity] == nil {
		return fmt.Err("appkit: unknown call target for module", call.Module, "entity", call.Entity)
	}
	target, ok := r.targets[call.Module][call.Entity][call.Call]
	if !ok || target.fn == nil {
		return fmt.Err("appkit: unknown call func for call", call.Call)
	}
	if target.store == nil || target.newList == nil {
		return fmt.Err("appkit: missing store or newList for entity", call.Entity)
	}
	row := target.newList().Append().(model.Model)
	if err := target.store.ReadOne(row, call.RecordId); err != nil {
		return err
	}
	fieldsToSave, err := target.fn(crud.CurrentUser{}, row, crud.IdempotencyKey(key))
	if err != nil {
		return err
	}
	if len(fieldsToSave) > 0 {
		if err := target.store.UpdateFields(row, call.RecordId, []string(fieldsToSave)); err != nil {
			return err
		}
	}
	return nil
}

type callInspector struct {
	entities []crud.StoredEntity
}

func (ci *callInspector) AddStoredEntity(entity crud.StoredEntity) {
	ci.entities = append(ci.entities, entity)
}

func (ci *callInspector) AddReadOnlyEntity(m model.ModuleNaming, newFilter func() model.Model, read func(c crud.CurrentUser, args crud.ReadArgs, filter model.Model) (model.ModelSlice, error)) crud.Route {
	return inertRoute{}
}

type inertRoute struct{}

func (inertRoute) RequiresMFA() crud.Route     { return inertRoute{} }
func (inertRoute) Describe(text string) crud.Route { return inertRoute{} }

func hasExternalCalls(ops []crud.Module) bool {
	for _, m := range ops {
		if m == nil {
			continue
		}
		for _, e := range m.Definition().Entities {
			if len(e.ExternalCalls) > 0 {
				return true
			}
		}
	}
	return false
}

// StartCallExecutor starts a background timer that periodically processes due external calls and recovers expired ones.
// It returns a function to cancel the background execution.
//
// Example: stop := appkit.StartCallExecutor(queue, runner, db, 5*time.Second, logger)
//	defer stop()
func StartCallExecutor(q *externalcalls.Queue, r externalcalls.Runner, db *orm.DB, interval time.Duration, logger func(...any)) func() {
	var stopped bool
	var mu sync.Mutex
	var activeTimer time.Timer

	var scheduleNext func()
	scheduleNext = func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		activeTimer = time.AfterFunc(int(interval.Milliseconds()), func() {
			runDueCalls(q, r, db, logger)
			scheduleNext()
		})
	}

	scheduleNext()

	stopFn := func() {
		mu.Lock()
		defer mu.Unlock()
		stopped = true
		if activeTimer != nil {
			activeTimer.Stop()
		}
	}

	executorMu.Lock()
	if executorStop != nil {
		executorStop()
	}
	executorStop = stopFn
	executorMu.Unlock()

	return stopFn
}

// StopCallExecutor stops the active background call executor, if running.
//
// Example: appkit.StopCallExecutor()
func StopCallExecutor() {
	executorMu.Lock()
	defer executorMu.Unlock()
	if executorStop != nil {
		executorStop()
		executorStop = nil
	}
}

func runDueCalls(q *externalcalls.Queue, runner externalcalls.Runner, db *orm.DB, logger func(...any)) {
	recoverExpiredReplies(q, runner, db, logger)

	report, err := q.RunPendingCalls(runner, 50)
	if err != nil {
		if logger != nil {
			logger("appkit: externalcalls error:", err)
		}
		return
	}
	if (report.Done > 0 || report.Retried > 0 || report.Failed > 0) && logger != nil {
		logger("appkit: externalcalls:", "done:", report.Done, "retried:", report.Retried, "failed:", report.Failed)
	}
}

func recoverExpiredReplies(q *externalcalls.Queue, runner externalcalls.Runner, db *orm.DB, logger func(...any)) {
	row := &externalcalls.QueuedCall{}
	query := db.Query(row).
		Where(externalcalls.QueuedCall_.State).Eq(string(externalcalls.StateQueued)).
		Where(externalcalls.QueuedCall_.Mode).Eq(int64(model.WaitForReply))
	list, err := externalcalls.ReadAllQueuedCall(query)
	if err != nil {
		return
	}
	now := time.NowNano()
	for _, c := range list {
		def, ok := runner.Declaration(c.Module, c.Entity, c.Call)
		if !ok {
			_ = q.MarkFailedAttempt(c.Id, model.ExternalCallDefinition{Name: c.Call, Mode: model.WaitForReply}, "call declaration not found")
			continue
		}
		maxWait := time.Duration(def.MaxWaitSeconds) * time.Second
		if now > c.CreatedAt.Add(maxWait) {
			_ = q.MarkFailedAttempt(c.Id, def, "timeout")
		}
	}
}
