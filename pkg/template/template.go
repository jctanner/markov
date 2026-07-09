package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flosch/pongo2/v6"
)

func init() {
	pongo2.RegisterFilter("fromjson", filterFromJSON)
	pongo2.RegisterFilter("from_json", filterFromJSON)
	pongo2.RegisterFilter("trim", filterTrim)
	pongo2.RegisterFilter("to_json", filterToJSON)
	pongo2.RegisterFilter("tojson", filterToJSON)
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

func filterTrim(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	return pongo2.AsValue(strings.TrimSpace(in.String())), nil
}

func filterToJSON(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
	data, err := json.Marshal(in.Interface())
	if err != nil {
		return nil, &pongo2.Error{
			Sender:    "filter:to_json",
			OrigError: err,
		}
	}
	return pongo2.AsSafeValue(string(data)), nil
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
		if rendered, ok, err := e.renderNativeExpression(val, ctx); ok || err != nil {
			return rendered, err
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

func (e *Engine) renderNativeExpression(tmpl string, ctx map[string]any) (any, bool, error) {
	expr, ok := exactExpression(tmpl)
	if !ok {
		return nil, false, nil
	}

	parts := strings.Split(expr, "|")
	path := strings.TrimSpace(parts[0])
	if path == "" {
		return nil, false, nil
	}
	value, ok := lookupPath(ctx, path)
	if !ok {
		return nil, false, nil
	}

	for _, rawFilter := range parts[1:] {
		filter := strings.TrimSpace(rawFilter)
		switch filter {
		case "fromjson", "from_json":
			parsed, err := parseJSONValue(value)
			if err != nil {
				return nil, true, fmt.Errorf("from_json: %w", err)
			}
			value = parsed
		case "tojson", "to_json":
			data, err := json.Marshal(value)
			if err != nil {
				return nil, true, fmt.Errorf("to_json: %w", err)
			}
			value = string(data)
		default:
			return nil, false, nil
		}
	}

	if len(parts) == 1 {
		if parsed, err := parseJSONObjectOrArrayString(value); err == nil {
			value = parsed
		}
	}
	return value, true, nil
}

func exactExpression(tmpl string) (string, bool) {
	s := strings.TrimSpace(tmpl)
	if !strings.HasPrefix(s, "{{") || !strings.HasSuffix(s, "}}") {
		return "", false
	}
	inner := strings.TrimSpace(s[2 : len(s)-2])
	if inner == "" || strings.Contains(inner, "{") || strings.Contains(inner, "}") {
		return "", false
	}
	return inner, true
}

func lookupPath(ctx map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = ctx
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func parseJSONValue(value any) (any, error) {
	s, ok := value.(string)
	if !ok {
		return value, nil
	}
	s = strings.TrimSpace(s)
	var parsed any
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func parseJSONObjectOrArrayString(value any) (any, error) {
	s, ok := value.(string)
	if !ok {
		return value, nil
	}
	s = strings.TrimSpace(s)
	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return value, fmt.Errorf("value is not a JSON object or array")
	}
	return parseJSONValue(s)
}
