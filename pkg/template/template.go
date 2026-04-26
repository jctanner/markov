package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flosch/pongo2/v6"
)

func init() {
	pongo2.RegisterFilter("fromjson", filterFromJSON)
}

func filterFromJSON(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	raw := in.String()
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:fromjson",
			OrigError: err,
		}
	}
	return pongo2.AsValue(parsed), nil
}

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Render(tmpl string, ctx map[string]any) (string, error) {
	t, err := pongo2.FromString(tmpl)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	result, err := t.Execute(pongo2.Context(ctx))
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return result, nil
}

func (e *Engine) RenderMap(params map[string]any, ctx map[string]any) (map[string]any, error) {
	result := make(map[string]any)
	for k, v := range params {
		rendered, err := e.renderValue(v, ctx)
		if err != nil {
			return nil, fmt.Errorf("rendering param %q: %w", k, err)
		}
		result[k] = rendered
	}
	return result, nil
}

func (e *Engine) EvalBool(expr string, ctx map[string]any) (bool, error) {
	wrapped := fmt.Sprintf("{%% if %s %%}true{%% endif %%}", expr)
	t, err := pongo2.FromString(wrapped)
	if err != nil {
		return false, fmt.Errorf("parsing expression %q: %w", expr, err)
	}
	result, err := t.Execute(pongo2.Context(ctx))
	if err != nil {
		return false, fmt.Errorf("evaluating expression %q: %w", expr, err)
	}
	return strings.TrimSpace(result) == "true", nil
}

func (e *Engine) renderValue(v any, ctx map[string]any) (any, error) {
	switch val := v.(type) {
	case string:
		if !strings.Contains(val, "{{") && !strings.Contains(val, "{%") {
			return val, nil
		}
		return e.Render(val, ctx)
	case map[string]any:
		return e.RenderMap(val, ctx)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			rendered, err := e.renderValue(item, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil
	default:
		return v, nil
	}
}
