package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAnsible writes an executable that records its argv, working directory,
// and any -i/-e file contents, then prints canned output.
func fakeAnsible(t *testing.T, stdout string, exit int) (bin, record string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "fake-ansible")
	record = filepath.Join(dir, "record")
	script := "#!/bin/sh\n" +
		"{ pwd; for a in \"$@\"; do echo \"ARG:$a\"; done\n" +
		"prev=; for a in \"$@\"; do case \"$prev\" in -i|-e) case \"$a\" in @*) cat \"${a#@}\";; /*) cat \"$a\";; esac;; esac; prev=$a; done\n" +
		"echo \"COLOR:$ANSIBLE_NOCOLOR CUSTOM:$CUSTOM\"; } > " + record + "\n" +
		"cat <<'EOF'\n" + stdout + "\nEOF\nexit " + string(rune('0'+exit)) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return bin, record
}

func readRecord(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const sampleRecap = `PLAY [all] ***

PLAY RECAP *********************************************************************
web01                      : ok=3    changed=1    unreachable=0    failed=0    skipped=2    rescued=0    ignored=0
web02                      : ok=2    changed=0    unreachable=0    failed=1    skipped=0    rescued=0    ignored=0
`

func TestAnsiblePlaybookBuildsCommandAndParsesRecap(t *testing.T) {
	bin, record := fakeAnsible(t, sampleRecap, 0)
	workdir := t.TempDir()
	result, err := NewAnsiblePlaybook().Execute(context.Background(), map[string]any{
		"binary":     bin,
		"chdir":      workdir,
		"playbook":   "site.yml",
		"inventory":  []any{"hosts.ini", "web01,web02,"},
		"limit":      []any{"web01", "web02"},
		"tags":       "deploy",
		"skip_tags":  []any{"slow", "debug"},
		"extra_vars": map[string]any{"version": "1.2.3"},
		"check":      true,
		"become":     true,
		"forks":      10,
		"verbosity":  2,
		"extra_args": []any{"--flush-cache"},
		"env":        map[string]any{"CUSTOM": "yes"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	rec := readRecord(t, record)
	resolved, _ := filepath.EvalSymlinks(workdir)
	for _, want := range []string{
		resolved + "\n",
		"ARG:-i\nARG:hosts.ini\nARG:-i\nARG:web01,web02,\n",
		"ARG:--limit\nARG:web01,web02\n",
		"ARG:--tags\nARG:deploy\n",
		"ARG:--skip-tags\nARG:slow,debug\n",
		"ARG:--check\n", "ARG:--become\n",
		"ARG:--forks\nARG:10\n", "ARG:-vv\n", "ARG:--flush-cache\n",
		`{"version":"1.2.3"}`,
		"COLOR:1 CUSTOM:yes",
	} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
	var lastArg string
	for _, line := range strings.Split(rec, "\n") {
		if strings.HasPrefix(line, "ARG:") {
			lastArg = line
		}
	}
	if lastArg != "ARG:site.yml" {
		t.Errorf("playbook should be the last argument, got %q", lastArg)
	}
	if strings.Contains(rec, "version=") || strings.Contains(rec, "ARG:{") {
		t.Errorf("extra_vars map must not be passed inline:\n%s", rec)
	}

	if got := result.Output["changed"]; got != true {
		t.Errorf("changed = %v, want true", got)
	}
	recap := result.Output["recap"].(map[string]any)
	web01 := recap["web01"].(map[string]any)
	if web01["ok"] != 3 || web01["changed"] != 1 || web01["skipped"] != 2 {
		t.Errorf("web01 recap = %v", web01)
	}
	if recap["web02"].(map[string]any)["failed"] != 1 {
		t.Errorf("web02 recap = %v", recap["web02"])
	}
}

func TestAnsiblePlaybookInlineInventoryMapUsesTempFileAndCleansUp(t *testing.T) {
	bin, record := fakeAnsible(t, sampleRecap, 0)
	_, err := NewAnsiblePlaybook().Execute(context.Background(), map[string]any{
		"binary":   bin,
		"playbook": "site.yml",
		"inventory": map[string]any{
			"all": map[string]any{"hosts": map[string]any{"web01": map[string]any{"ansible_host": "10.0.0.1"}}},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	rec := readRecord(t, record)
	if !strings.Contains(rec, "ansible_host: 10.0.0.1") {
		t.Errorf("inventory content not visible to ansible:\n%s", rec)
	}
	for _, line := range strings.Split(rec, "\n") {
		if p, ok := strings.CutPrefix(line, "ARG:/"); ok {
			if _, err := os.Stat("/" + p); err == nil {
				t.Errorf("temp file %q was not removed", p)
			}
		}
	}
}

func TestAnsiblePlaybookFailureReturnsOutput(t *testing.T) {
	bin, _ := fakeAnsible(t, sampleRecap, 2)
	result, err := NewAnsiblePlaybook().Execute(context.Background(), map[string]any{
		"binary": bin, "playbook": "site.yml",
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
	if result == nil || result.Output["exit_code"] != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestAnsibleAdhocBuildsCommand(t *testing.T) {
	bin, record := fakeAnsible(t, "web01 | CHANGED | rc=0 >>\nok", 0)
	result, err := NewAnsible().Execute(context.Background(), map[string]any{
		"binary":      bin,
		"pattern":     "web",
		"inventory":   "hosts.ini",
		"module":      "copy",
		"module_args": map[string]any{"src": "a", "dest": "/b"},
		"become":      true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	rec := readRecord(t, record)
	for _, want := range []string{"ARG:web\n", "ARG:-m\nARG:copy\n", `ARG:-a` + "\n" + `ARG:{"dest":"/b","src":"a"}`, "ARG:--become\n"} {
		if !strings.Contains(rec, want) {
			t.Errorf("record missing %q:\n%s", want, rec)
		}
	}
	if result.Output["changed"] != true {
		t.Errorf("changed = %v", result.Output["changed"])
	}
}

func TestAnsibleAdhocDefaultsToCommandModule(t *testing.T) {
	bin, record := fakeAnsible(t, "", 0)
	if _, err := NewAnsible().Execute(context.Background(), map[string]any{
		"binary": bin, "pattern": "all", "module_args": "uptime",
	}); err != nil {
		t.Fatal(err)
	}
	rec := readRecord(t, record)
	if !strings.Contains(rec, "ARG:-m\nARG:command\nARG:-a\nARG:uptime\n") {
		t.Errorf("unexpected args:\n%s", rec)
	}
}

func TestAnsibleRejectsInvalidParams(t *testing.T) {
	for name, tc := range map[string]struct {
		exec   Executor
		params map[string]any
	}{
		"playbook missing":     {NewAnsiblePlaybook(), map[string]any{}},
		"pattern missing":      {NewAnsible(), map[string]any{}},
		"bad inventory type":   {NewAnsiblePlaybook(), map[string]any{"playbook": "p", "inventory": 3}},
		"bad extra_vars type":  {NewAnsiblePlaybook(), map[string]any{"playbook": "p", "extra_vars": []any{"x"}}},
		"bad verbosity":        {NewAnsiblePlaybook(), map[string]any{"playbook": "p", "verbosity": 9}},
		"bad check type":       {NewAnsiblePlaybook(), map[string]any{"playbook": "p", "check": "yes"}},
		"chdir not a dir":      {NewAnsiblePlaybook(), map[string]any{"playbook": "p", "chdir": "/nonexistent-dir"}},
		"bad module_args type": {NewAnsible(), map[string]any{"pattern": "all", "module_args": 5}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tc.exec.Execute(context.Background(), tc.params); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
