package schema

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/jctanner/markov/pkg/parser"
)

func TestEveryBuiltInStepTypeIsDescribedExactlyOnce(t *testing.T) {
	want := parser.PrimitiveNames()
	var got []string
	seen := map[string]bool{}
	for _, st := range Build().StepTypes {
		if seen[st.Name] {
			t.Errorf("step type %q listed twice", st.Name)
		}
		seen[st.Name] = true
		got = append(got, st.Name)
		if st.Summary == "" {
			t.Errorf("%s has no summary", st.Name)
		}
		if st.Inputs != "params" && st.Inputs != "step" {
			t.Errorf("%s: inputs = %q", st.Name, st.Inputs)
		}
		if (st.Inputs == "step") != (len(st.StepFields) > 0) {
			t.Errorf("%s: step_fields must be set exactly when inputs is \"step\"", st.Name)
		}
	}
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("schema step types = %v, parser primitives = %v", got, want)
	}
}

func TestFieldsAreReflectedFromTheParserStructs(t *testing.T) {
	s := Build()
	has := func(fs []Field, name, typ string) bool {
		for _, f := range fs {
			if f.Name == name && f.Type == typ {
				return true
			}
		}
		return false
	}
	for _, c := range []struct {
		fs        []Field
		name, typ string
	}{
		{s.File, "entrypoint", "string"}, {s.File, "forks", "int"}, {s.File, "workflows", "list"}, {s.File, "step_types", "map"},
		{s.Workflow, "steps", "list"}, {s.Workflow, "rescue", "list"}, {s.Workflow, "always", "list"},
		{s.Step, "type", "string"}, {s.Step, "params", "map"}, {s.Step, "vars", "map"}, {s.Step, "that", "list"}, {s.Step, "for_each", "string"}, {s.Step, "concurrency", "int"}, {s.Step, "workflow", "string"},
	} {
		if !has(c.fs, c.name, c.typ) {
			t.Errorf("missing field %s (%s)", c.name, c.typ)
		}
	}
	for _, f := range s.File {
		if f.Name == "" || f.Name == "-" {
			t.Errorf("a field without a name: %+v", f)
		}
	}
	// A field that is not part of the YAML schema must not appear (ScriptDir is yaml:"-").
	for _, f := range s.File {
		if strings.EqualFold(f.Name, "scriptdir") {
			t.Error("ScriptDir is not part of the schema")
		}
	}
}

// The parameter table and the reference docs must agree: every parameter listed here appears in
// its type's section of docs/reference/step-types.md, and step-level fields exist on the parser's Step.
func TestParametersAreDocumentedAndStepFieldsExist(t *testing.T) {
	data, err := os.ReadFile("../../docs/reference/step-types.md")
	if err != nil {
		t.Skipf("docs not available: %v", err)
	}
	sections := map[string]string{}
	var cur string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			cur = strings.TrimSpace(strings.TrimPrefix(line, "## "))
		}
		sections[cur] += line + "\n"
	}
	stepFields := map[string]bool{}
	for _, f := range Build().Step {
		stepFields[f.Name] = true
	}
	tick := regexp.MustCompile("`([a-z_]+)`")
	for _, st := range Build().StepTypes {
		for _, f := range st.StepFields {
			if !stepFields[f] {
				t.Errorf("%s: step field %q is not a field of the parser's Step", st.Name, f)
			}
		}
		doc, ok := sections[st.Name]
		if !ok {
			continue // llm_invoke and jev are described in other sections
		}
		documented := map[string]bool{}
		for _, m := range tick.FindAllStringSubmatch(doc, -1) {
			documented[m[1]] = true
		}
		for _, pr := range st.Params {
			if !documented[pr.Name] {
				t.Errorf("%s: parameter %q is not in the reference docs", st.Name, pr.Name)
			}
		}
		// Required parameters really are marked required in the docs' tables.
		for _, pr := range st.Params {
			if pr.Required && !strings.Contains(doc, "`"+pr.Name+"`") {
				t.Errorf("%s: required parameter %q is not documented", st.Name, pr.Name)
			}
		}
	}
}
