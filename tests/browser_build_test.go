//go:build !wasm && !browser

package tests

import (
	"os/exec"
	"strings"
	"testing"
)

func TestBrowserBuild_AppkitImportsNoServerCode(t *testing.T) {
	cmd := exec.Command("go", "list", "-tags", "browser", "-f", "{{join .Imports \"\\n\"}}", ".")
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
		"webtyp.com/server/httpd",
		"webtyp.com/externalcalls",
		"webtyp.com/backoff",
		"webtyp.com/unixid",
		"webtyp.com/rpc",
		"webtyp.com/events/inprocess",
	}
	output := "\n" + string(out) + "\n"
	for _, f := range forbidden {
		if strings.Contains(output, "\n"+f+"\n") {
			t.Errorf("appkit's browser files must not import %s", f)
		}
	}
}
