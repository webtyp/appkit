//go:build !wasm

package tests

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"webtyp.com/appkit"
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/sqlite"
	"webtyp.com/storage"
)

// holiday is a model whose table the test creates by hand with one more column, "label".
type holiday struct{}

func (*holiday) ModelName() string { return "holiday" }
func (*holiday) Schema() []model.Field {
	return []model.Field{{Name: "id", Type: model.Text()}, {Name: "name", Type: model.Text()}}
}
func (*holiday) Pointers() []any                { return nil }
func (h *holiday) IsNil() bool                  { return h == nil }
func (*holiday) EncodeFields(model.FieldWriter) {}
func (*holiday) DecodeFields(model.FieldReader) {}

var holidaySchema = ddl.Schema{
	Steps: []ddl.Step{{Name: "calendar", Run: func(conn ddl.Execer, c ddl.Compiler) error {
		return ddl.New(conn, c).Sync(&holiday{})
	}}},
	Removed: []ddl.Removed{{Table: "holiday", Column: "label", Note: "moved to name"}},
}

// holidayDSN is a sqlite file with table holiday(id, name, label) and two rows with a label.
func holidayDSN(t *testing.T) string {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "app.db")
	conn, err := sqlite.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for _, q := range []string{
		"CREATE TABLE holiday (id TEXT PRIMARY KEY, name TEXT, label TEXT)",
		"INSERT INTO holiday (id, name, label) VALUES ('h1', NULL, 'Navidad')",
		"INSERT INTO holiday (id, name, label) VALUES ('h2', 'Año nuevo', 'Year')",
	} {
		if err := conn.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	return dsn
}

// logLines builds an app whose log is kept, one string per call.
func logLines(open func(string) (storage.Conn, error)) (appkit.App, *[]string) {
	var lines []string
	return appkit.App{
		Timezone: "America/Santiago",
		Server:   appkit.AppServer{Database: open},
		Log:      func(a ...any) { lines = append(lines, fmt.Sprintln(a...)) },
	}, &lines
}

func linesWith(lines []string, part string) []string {
	var out []string
	for _, l := range lines {
		if strings.Contains(l, part) {
			out = append(out, l)
		}
	}
	return out
}

func TestLeftovers_ServerWarnsAtStart(t *testing.T) {
	dsn := holidayDSN(t)
	app, lines := logLines(sqlite.Open)
	if _, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: dsn}, holidaySchema); err != nil {
		t.Fatal(err)
	}
	got := linesWith(*lines, "holiday.label")
	if len(got) != 1 || !strings.Contains(got[0], "leftover column") || !strings.Contains(got[0], "rows: 2") ||
		!strings.Contains(got[0], "note: moved to name") {
		t.Fatalf("want one leftover line for holiday.label with 2 rows and its note, got %q", *lines)
	}
	conn, err := sqlite.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cols, err := conn.(ddl.TableIntrospector).TableColumns("holiday")
	if err != nil || len(cols) != 3 {
		t.Fatalf("the server must never drop a column: got %v, %v", cols, err)
	}
}

func TestLeftovers_ServerStartsWhenUncheckable(t *testing.T) {
	app, lines := logLines(openMem)
	if _, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, holidaySchema); err != nil {
		t.Fatalf("a database that cannot be checked must not stop the server, got %v", err)
	}
	if len(linesWith(*lines, "leftover columns not checked")) != 1 {
		t.Fatalf("want one 'not checked' line, got %q", *lines)
	}
}

func TestLeftovers_NoSchemaNoCheck(t *testing.T) {
	app, lines := logLines(openMem)
	if _, err := appkit.BuildServer(app, appkit.ServerEnv{DSN: "mem"}, ddl.Schema{}); err != nil {
		t.Fatal(err)
	}
	if got := linesWith(*lines, "leftover"); len(got) != 0 {
		t.Fatalf("no steps: nothing to check, got %q", got)
	}
}
