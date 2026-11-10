//go:build !wasm

package tests

import (
	"path/filepath"
	"sync"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/backoff"
	"webtyp.com/crud"
	"webtyp.com/ddl"
	"webtyp.com/externalcalls"
	externalcallsmigrate "webtyp.com/externalcalls/migrate"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/sqlite"
	"webtyp.com/storage"
	"webtyp.com/time"
	"webtyp.com/unixid"
)

type testCallRec struct {
	ID     string
	Amount int64
	Status string
}

func (r *testCallRec) ModelName() string   { return "test_record" }
func (r *testCallRec) Allow() model.Action { return model.Create | model.Read | model.Update }
func (r *testCallRec) IsNil() bool         { return r == nil }
func (r *testCallRec) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "amount", Type: model.Int()},
		{Name: "status", Type: model.Text()},
	}
}
func (r *testCallRec) Pointers() []any {
	return []any{&r.ID, &r.Amount, &r.Status}
}
func (r *testCallRec) EncodeFields(w model.FieldWriter) {
	w.String("id", r.ID)
	w.Int("amount", r.Amount)
	w.String("status", r.Status)
}
func (r *testCallRec) DecodeFields(rd model.FieldReader) {
	if v, ok := rd.String("id"); ok {
		r.ID = v
	}
	if v, ok := rd.Int("amount"); ok {
		r.Amount = v
	}
	if v, ok := rd.String("status"); ok {
		r.Status = v
	}
}

type testCallRecList []testCallRec

func (l *testCallRecList) Append() model.Fielder  { *l = append(*l, testCallRec{}); return &(*l)[len(*l)-1] }
func (l *testCallRecList) Len() int               { return len(*l) }
func (l *testCallRecList) At(i int) model.Fielder { return &(*l)[i] }
func (l *testCallRecList) EncodeFields(w model.FieldWriter) {
	a := w.Array("records", len(*l))
	for i := range *l {
		a.Object(&(*l)[i])
	}
}
func (l *testCallRecList) DecodeFields(rd model.FieldReader) {
	if a, ok := rd.Array("records"); ok {
		for i := 0; i < a.Len(); i++ {
			f := l.Append().(model.Decodable)
			a.Object(i, f)
		}
	}
}
func (l *testCallRecList) IsNil() bool { return l == nil }

type memTestStore struct {
	mu   sync.Mutex
	rows map[string]*testCallRec
}

func newMemTestStore() *memTestStore {
	return &memTestStore{rows: make(map[string]*testCallRec)}
}

func (s *memTestStore) NewID() string { return "r-1" }
func (s *memTestStore) ReadOne(rec model.Model, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return model.NotFound("not found")
	}
	t := rec.(*testCallRec)
	t.ID = row.ID
	t.Amount = row.Amount
	t.Status = row.Status
	return nil
}
func (s *memTestStore) ReadAll(newRow func() model.Model, q crud.Query, onRow func(model.Model)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range s.rows {
		r := newRow().(*testCallRec)
		r.ID = row.ID
		r.Amount = row.Amount
		r.Status = row.Status
		onRow(r)
	}
	return nil
}
func (s *memTestStore) Create(rec model.Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := rec.(*testCallRec)
	s.rows[t.ID] = &testCallRec{ID: t.ID, Amount: t.Amount, Status: t.Status}
	return nil
}
func (s *memTestStore) UpdateFields(rec model.Model, id string, fields []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return model.NotFound("not found")
	}
	t := rec.(*testCallRec)
	for _, f := range fields {
		switch f {
		case "amount":
			row.Amount = t.Amount
		case "status":
			row.Status = t.Status
		}
	}
	return nil
}
func (s *memTestStore) Delete(rec model.Model, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, id)
	return nil
}

type testExternalCallModule struct {
	store *memTestStore
	calls map[string]int
}

func (m *testExternalCallModule) ModelName() string { return "test_module" }
func (m *testExternalCallModule) Definition() model.ModuleDefinition {
	return model.ModuleDefinition{
		Name: "test_module",
		Entities: []model.Definition{
			{
				Name:  "test_record",
				Allow: model.Create | model.Read | model.Update,
				Rules: model.Rules{Create: model.HaveNoBusinessRule, Read: model.HaveNoBusinessRule, Update: model.HaveNoBusinessRule},
				ExternalCalls: []model.ExternalCallDefinition{
					{Name: "charge", Mode: model.WaitForReply, MaxWaitSeconds: 10},
					{Name: "receipt", Mode: model.RetryInBackground, MaxAttempts: 3},
					{Name: "confirm", Mode: model.WaitForConfirmation},
				},
				Fields: model.Fields{
					{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
					{Name: "amount", Type: model.Int()},
					{Name: "status", Type: model.Text()},
				},
			},
		},
	}
}

func (m *testExternalCallModule) MountOperations(reg crud.Registry) {
	reg.AddStoredEntity(crud.StoredEntity{
		Model:         &testCallRec{},
		NewList:       func() model.ModelSlice { return &testCallRecList{} },
		Store:         m.store,
		Rules:         model.Rules{Create: model.HaveNoBusinessRule, Read: model.HaveNoBusinessRule, Update: model.HaveNoBusinessRule},
		ExternalCalls: m.Definition().Entities[0].ExternalCalls,
		Calls: map[string]crud.ExternalCallFunc{
			"charge": func(a crud.CurrentUser, rec model.Model, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
				m.calls["charge"]++
				p := rec.(*testCallRec)
				p.Status = "charged"
				return crud.Fields("status"), nil
			},
			"receipt": func(a crud.CurrentUser, rec model.Model, key crud.IdempotencyKey) (crud.FieldsToSave, error) {
				m.calls["receipt"]++
				p := rec.(*testCallRec)
				p.Status = "receipt_sent"
				return crud.Fields("status"), nil
			},
		},
	})
}

func setupTestDB(t *testing.T) (*orm.DB, model.IDGenerator, storage.Conn) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "appkit_test.db")
	conn, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	compiler, ok := conn.(ddl.Compiler)
	if !ok {
		t.Fatalf("conn does not implement ddl.Compiler")
	}
	if err := externalcallsmigrate.Migrate(conn, compiler); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	ids, err := unixid.NewUnixID()
	if err != nil {
		t.Fatalf("NewUnixID: %v", err)
	}
	return orm.New(conn), ids, conn
}

func TestCallQueueAdapter_AllMethods(t *testing.T) {
	db, ids, _ := setupTestDB(t)
	policy, err := backoff.New(time.Second, 10*time.Second)
	if err != nil {
		t.Fatalf("backoff.New: %v", err)
	}
	q, err := externalcalls.New(db, ids, time.NowNano, policy)
	if err != nil {
		t.Fatalf("externalcalls.New: %v", err)
	}
	adapter := appkit.NewCallQueue(q)
	if adapter == nil {
		t.Fatalf("expected non-nil adapter")
	}

	tx := orm.TenantStore(db, "t1", ids)
	callID, err := adapter.Add(tx, crud.CallRequest{
		TenantId: "t1",
		Module:   "test_module",
		Entity:   "test_record",
		RecordId: "rec-1",
		Call:     "charge",
		Mode:     model.WaitForReply,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if callID == "" {
		t.Fatalf("expected callID")
	}

	// MarkDone
	if err := adapter.MarkDone(callID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}

	// Add RetryInBackground call
	callID2, err := adapter.Add(tx, crud.CallRequest{
		TenantId: "t1",
		Module:   "test_module",
		Entity:   "test_record",
		RecordId: "rec-2",
		Call:     "receipt",
		Mode:     model.RetryInBackground,
	})
	if err != nil {
		t.Fatalf("Add receipt: %v", err)
	}

	// MarkFailedAttempt retries
	def := model.ExternalCallDefinition{Name: "receipt", Mode: model.RetryInBackground, MaxAttempts: 3}
	if err := adapter.MarkFailedAttempt(callID2, def, "net err"); err != nil {
		t.Fatalf("MarkFailedAttempt: %v", err)
	}

	// Add WaitForConfirmation call
	callID3, err := adapter.Add(tx, crud.CallRequest{
		TenantId: "t1",
		Module:   "test_module",
		Entity:   "test_record",
		RecordId: "rec-3",
		Call:     "confirm",
		Mode:     model.WaitForConfirmation,
	})
	if err != nil {
		t.Fatalf("Add confirm: %v", err)
	}

	if err := adapter.ConfirmCall(callID3); err != nil {
		t.Fatalf("ConfirmCall: %v", err)
	}

	callID4, err := adapter.Add(tx, crud.CallRequest{
		TenantId: "t1",
		Module:   "test_module",
		Entity:   "test_record",
		RecordId: "rec-4",
		Call:     "confirm",
		Mode:     model.WaitForConfirmation,
	})
	if err != nil {
		t.Fatalf("Add confirm 4: %v", err)
	}
	if err := adapter.RejectCall(callID4, "rejected"); err != nil {
		t.Fatalf("RejectCall: %v", err)
	}
}

func TestCallRunner_DeclarationAndRun(t *testing.T) {
	store := newMemTestStore()
	store.Create(&testCallRec{ID: "rec-1", Amount: 500, Status: "pending"})

	mod := &testExternalCallModule{
		store: store,
		calls: make(map[string]int),
	}

	runner := appkit.NewCallRunner([]crud.Module{mod})

	// Declaration check
	def, ok := runner.Declaration("test_module", "test_record", "charge")
	if !ok || def.Mode != model.WaitForReply {
		t.Fatalf("expected declaration for charge, got ok=%v def=%+v", ok, def)
	}

	_, ok = runner.Declaration("test_module", "test_record", "unknown")
	if ok {
		t.Fatalf("expected false for unknown call")
	}

	// Run check
	err := runner.Run(externalcalls.QueuedCall{
		Id:       "call-1",
		Module:   "test_module",
		Entity:   "test_record",
		RecordId: "rec-1",
		Call:     "charge",
	}, "call-1")
	if err != nil {
		t.Fatalf("runner.Run: %v", err)
	}

	if mod.calls["charge"] != 1 {
		t.Fatalf("expected charge called once, got %d", mod.calls["charge"])
	}

	var updated testCallRec
	if err := store.ReadOne(&updated, "rec-1"); err != nil {
		t.Fatalf("ReadOne: %v", err)
	}
	if updated.Status != "charged" {
		t.Fatalf("expected status 'charged', got %q", updated.Status)
	}
}

func TestBuildServer_WiresQueueAndRunner(t *testing.T) {
	defer appkit.StopCallExecutor()

	dbPath := filepath.Join(t.TempDir(), "buildserver_test.db")
	openFn := func(string) (storage.Conn, error) {
		conn, err := sqlite.Open(dbPath)
		if err != nil {
			return nil, err
		}
		if compiler, ok := conn.(ddl.Compiler); ok {
			_ = externalcallsmigrate.Migrate(conn, compiler)
		}
		return conn, nil
	}

	store := newMemTestStore()
	testMod := &testExternalCallModule{store: store, calls: make(map[string]int)}

	app := appkit.App{
		TenantID: "tenant-test",
		Timezone: "America/Santiago",
		Log:      func(...any) {},
		Server: appkit.AppServer{
			Database: openFn,
		},
		Modules: []appkit.Module{
			{
				Name: "test_module",
				Server: appkit.ModuleServer{
					Build: func(d appkit.Deps) (model.ModuleNaming, error) {
						if d.Queue == nil {
							t.Errorf("expected d.Queue to be non-nil in Deps")
						}
						return testMod, nil
					},
				},
			},
		},
	}

	srv, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: dbPath}, ddl.Schema{})
	if err != nil {
		t.Fatalf("BuildServer: %v", err)
	}
	if srv == nil {
		t.Fatalf("expected non-nil server")
	}

	if crud.DefaultQueue() == nil {
		t.Fatalf("expected crud.DefaultQueue to be populated")
	}
}

func TestCallExecutor_RecoversExpiredReplies(t *testing.T) {
	defer appkit.StopCallExecutor()
	db, ids, _ := setupTestDB(t)

	policy, _ := backoff.New(time.Second, 10*time.Second)
	q, _ := externalcalls.New(db, ids, time.NowNano, policy)

	store := newMemTestStore()
	mod := &testExternalCallModule{store: store, calls: make(map[string]int)}
	runner := appkit.NewCallRunner([]crud.Module{mod})

	// Add an expired WaitForReply call manually
	past := time.NowNano() - 100*time.Nano(time.Second)
	expiredRow := &externalcalls.QueuedCall{
		Id:            ids.NewID(),
		TenantId:      "t1",
		Module:        "test_module",
		Entity:        "test_record",
		RecordId:      "rec-1",
		Call:          "charge",
		Mode:          int64(model.WaitForReply),
		State:         string(externalcalls.StateQueued),
		NextAttemptAt: past,
		CreatedAt:     past,
		UpdatedAt:     past,
	}
	if err := db.Create(expiredRow); err != nil {
		t.Fatalf("db.Create: %v", err)
	}

	// Start executor which triggers runDueCalls and recovers expired replies
	stop := appkit.StartCallExecutor(q, runner, db, 50*time.Millisecond, nil)
	defer stop()

	time.AfterFunc(100, func() {}) // small tick

	// Verify the call is failed
	var readRow externalcalls.QueuedCall
	_, err := externalcalls.ReadOneQueuedCall(db.Query(&readRow).Where(externalcalls.QueuedCall_.Id).Eq(expiredRow.Id), &readRow)
	if err != nil {
		t.Fatalf("ReadOneQueuedCall: %v", err)
	}
	// Give recovery a moment to run
	for i := 0; i < 20 && readRow.State != string(externalcalls.StateFailed); i++ {
		_, _ = externalcalls.ReadOneQueuedCall(db.Query(&readRow).Where(externalcalls.QueuedCall_.Id).Eq(expiredRow.Id), &readRow)
		stdSleep(10)
	}

	if readRow.State != string(externalcalls.StateFailed) {
		t.Fatalf("expected state failed, got %q", readRow.State)
	}
	if readRow.LastError != "timeout" {
		t.Fatalf("expected lastError 'timeout', got %q", readRow.LastError)
	}
}

func stdSleep(ms int) {
	done := make(chan struct{})
	time.AfterFunc(ms, func() { close(done) })
	<-done
}
