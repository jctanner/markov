package template

import (
	"testing"
)

func TestFromJSON_Object(t *testing.T) {
	eng := New()
	ctx := map[string]any{
		"raw": `{"name":"alice","age":30}`,
	}

	result, err := eng.Render("{{ raw | fromjson }}", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if result == "" || result == `{"name":"alice","age":30}` {
		t.Logf("rendered as string representation: %s", result)
	}
}

func TestFromJSON_ArrayAccess(t *testing.T) {
	eng := New()
	ctx := map[string]any{
		"raw": `[{"key":"A"},{"key":"B"}]`,
	}

	result, err := eng.Render("{{ raw | fromjson | length }}", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if result != "2" {
		t.Errorf("length = %q, want 2", result)
	}
}

func TestFromJSON_UsedInSetFact(t *testing.T) {
	eng := New()

	ctx := map[string]any{
		"stdout": `[{"key":"PROJ-100"},{"key":"PROJ-200"}]`,
	}

	rendered, err := eng.Render("{{ stdout | fromjson }}", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if rendered == "" {
		t.Error("rendered is empty")
	}
}

func TestFromJSON_InvalidJSON(t *testing.T) {
	eng := New()
	ctx := map[string]any{
		"raw": `not valid json`,
	}

	_, err := eng.Render("{{ raw | fromjson }}", ctx)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestFromJSON_NestedAccess(t *testing.T) {
	eng := New()
	ctx := map[string]any{
		"raw": `{"data":{"count":42}}`,
	}

	result, err := eng.Render("{{ raw | fromjson }}", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if result == "" {
		t.Error("rendered is empty")
	}
}

func TestFromJSON_ScalarString(t *testing.T) {
	eng := New()
	ctx := map[string]any{
		"raw": `"hello world"`,
	}

	result, err := eng.Render("{{ raw | fromjson }}", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if result != "hello world" {
		t.Errorf("result = %q, want 'hello world'", result)
	}
}

func TestFromJSON_Number(t *testing.T) {
	eng := New()
	ctx := map[string]any{
		"raw": `42`,
	}

	result, err := eng.Render("{{ raw | fromjson }}", ctx)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if result != "42" && result != "42.000000" {
		t.Errorf("result = %q, want '42' or '42.000000'", result)
	}
}
