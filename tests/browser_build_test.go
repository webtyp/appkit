//go:build !wasm

package tests

import (
	"os/exec"
	"strings"
	"testing"
)

func TestBrowserBuild_LeavesServerCodeOut(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-tags", "browser", "-f", "{{.ImportPath}}", "./...")
	cmd.Dir = ".."
	cmd.Env = append(cmd.Environ(), "GOOS=js", "GOARCH=wasm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v (output: %s)", err, out)
	}
	forbidden := []string{
		"webtyp.com/orm",
		"webtyp.com/mcp",
		"webtyp.com/rbac",
		"webtyp.com/ddl",
		"webtyp.com/server/httpd",
		"webtyp.com/storage",
	}
	output := string(out)
	for _, f := range forbidden {
		if strings.Contains(output, f) {
			t.Errorf("browser build must not import %s, found in deps", f)
		}
	}
}
