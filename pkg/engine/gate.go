package engine

import (
	"fmt"
	"log"
	"sort"

	"github.com/jctanner/markov/pkg/parser"
)

type gateResult struct {
	Action     string
	FiredRules []string
	Facts      map[string]any
}

func (e *Engine) evaluateGate(runID string, ruleNames []string, gateFacts map[string]any, runCtx map[string]any) (*gateResult, error) {
	rules := make([]parser.Rule, 0, len(ruleNames))
	for _, name := range ruleNames {
		r := e.file.GetRule(name)
		if r == nil {
			return nil, fmt.Errorf("rule %q not found", name)
		}
		rules = append(rules, *r)
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Salience > rules[j].Salience
	})

	// Build evaluation context: vars + set_fact values + gate facts
	evalCtx := make(map[string]any)
	for k, v := range runCtx {
		evalCtx[k] = v
	}

	// Render and apply gate-level facts (maps step results to clean names)
	if len(gateFacts) > 0 {
		rendered, err := e.tmpl.RenderMap(gateFacts, runCtx)
		if err != nil {
			return nil, fmt.Errorf("rendering gate facts: %w", err)
		}
		for k, v := range rendered {
			evalCtx[k] = v
		}
	}

	fired := make(map[string]bool)
	var firedList []string
	gateAction := "continue"
	gateActionSalience := -1
	setFacts := make(map[string]any)
	maxCycles := 100

	for cycle := 0; cycle < maxCycles; cycle++ {
		changed := false

		for _, rule := range rules {
			if fired[rule.Name] {
				continue
			}

			ok, err := e.tmpl.EvalBool(rule.When, evalCtx)
			if err != nil {
				return nil, fmt.Errorf("rule %q: evaluating condition: %w", rule.Name, err)
			}
			if !ok {
				continue
			}

			fired[rule.Name] = true
			firedList = append(firedList, rule.Name)

			action := rule.Action
			if action == "" {
				action = "continue"
			}

			e.verbose("[run:%s]   rule %q fired (salience=%d, action=%s)", runID, rule.Name, rule.Salience, action)

			if rule.Salience > gateActionSalience {
				gateAction = action
				gateActionSalience = rule.Salience
			}

			for k, v := range rule.SetFact {
				evalCtx[k] = v
				setFacts[k] = v
				changed = true
				e.verbose("[run:%s]     set %s = %v", runID, k, v)
			}
		}

		if !changed {
			break
		}
	}

	// Merge set facts back into runCtx
	for k, v := range setFacts {
		runCtx[k] = v
	}

	// Merge gate-level facts into runCtx
	if len(gateFacts) > 0 {
		rendered, _ := e.tmpl.RenderMap(gateFacts, runCtx)
		for k, v := range rendered {
			runCtx[k] = v
		}
	}

	if len(firedList) == 0 {
		log.Printf("[run:%s]   no rules matched", runID)
	}

	return &gateResult{
		Action:     gateAction,
		FiredRules: firedList,
		Facts:      setFacts,
	}, nil
}
