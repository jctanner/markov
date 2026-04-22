package engine

import (
	"fmt"
	"strings"
)

func (e *Engine) evalFacts(vars map[string]any, runCtx map[string]any) (map[string]any, error) {
	result := make(map[string]any)

	for k, v := range vars {
		val, err := e.evalFact(v, runCtx)
		if err != nil {
			return nil, fmt.Errorf("fact %q: %w", k, err)
		}
		result[k] = val
		runCtx[k] = val
	}

	return result, nil
}

func (e *Engine) evalFact(v any, runCtx map[string]any) (any, error) {
	expr, ok := v.(string)
	if !ok {
		return v, nil
	}

	if strings.Contains(expr, "{{") || strings.Contains(expr, "{%") {
		rendered, err := e.tmpl.Render(expr, runCtx)
		if err != nil {
			return nil, fmt.Errorf("rendering template: %w", err)
		}
		return rendered, nil
	}

	val, err := e.tmpl.EvalBool(expr, runCtx)
	if err != nil {
		return nil, fmt.Errorf("evaluating expression: %w", err)
	}
	return val, nil
}
