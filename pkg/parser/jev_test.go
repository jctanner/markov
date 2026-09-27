package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const jevWorkflow = `
entrypoint: main
connections:
  local: {type: jev, base_url: http://localhost:18009, api_key_env: KEV_API_KEY}
decisions:
  risk: {connection: local, type: noul, instructions: "Is this risky?"}
rules:
  - {name: review, when: "risk > 0.7", action: pause}
workflows:
  - name: main
    steps:
      - name: gate
        type: gate
        facts:
          risk: {decision: risk, state: hello, select: noul}
        rules: [review]
`

func TestJevSchema(t *testing.T) {
	if _, err := Parse([]byte(jevWorkflow)); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{"api_key_env: KEV_API_KEY", "api_key: secret"},
		{"connection: local", "connection: missing"},
		{"decision: risk", "decision: missing"},
		{"select: noul", "select: confidence"},
		{"type: noul", "type: bogus"},
		{"state: hello", "states: hello"},
		{"http://localhost:18009", "file:///tmp/endpoint"},
	} {
		if _, err := Parse([]byte(strings.Replace(jevWorkflow, pair[0], pair[1], 1))); err == nil {
			t.Errorf("accepted %s", pair[1])
		}
	}
}

func TestDirectoryJevDefinitions(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "workflows"), 0755)
	for name, body := range map[string]string{
		"meta.yaml": "entrypoint: main\n",
		"vars.yaml": "{}\n", "rules.yaml": "[]\n", "step_types.yaml": "{}\n",
		"connections.yaml":    "local: {type: jev, base_url: http://localhost:18009}\n",
		"decisions.yaml":      "risk: {connection: local, type: noul}\n",
		"workflows/main.yaml": "name: main\nsteps: []\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	wf, err := ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if wf.Decisions["risk"].Connection != "local" {
		t.Fatal("decision not loaded")
	}
}
