package parser

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ParseFile(path string) (*WorkflowFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file: %w", err)
	}
	return Parse(data)
}

func Parse(data []byte) (*WorkflowFile, error) {
	var wf WorkflowFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow YAML: %w", err)
	}
	if err := loadRuleFiles(&wf); err != nil {
		return nil, err
	}
	if err := validate(&wf); err != nil {
		return nil, err
	}
	return &wf, nil
}

func validate(wf *WorkflowFile) error {
	if wf.Forks <= 0 {
		wf.Forks = 5
	}
	if len(wf.Workflows) == 0 {
		return fmt.Errorf("no workflows defined")
	}
	if wf.Entrypoint == "" {
		return fmt.Errorf("entrypoint is required")
	}
	if !hasWorkflow(wf, wf.Entrypoint) {
		return fmt.Errorf("entrypoint %q not found in workflows", wf.Entrypoint)
	}

	names := make(map[string]bool)
	for _, w := range wf.Workflows {
		if w.Name == "" {
			return fmt.Errorf("workflow missing name")
		}
		if names[w.Name] {
			return fmt.Errorf("duplicate workflow name %q", w.Name)
		}
		names[w.Name] = true
		if err := validateSteps(wf, w.Name, w.Steps); err != nil {
			return err
		}
	}

	return nil
}

func validateSteps(wf *WorkflowFile, workflowName string, steps []Step) error {
	stepNames := make(map[string]bool)
	for _, s := range steps {
		if s.Name == "" {
			return fmt.Errorf("workflow %q: step missing name", workflowName)
		}
		if stepNames[s.Name] {
			return fmt.Errorf("workflow %q: duplicate step name %q", workflowName, s.Name)
		}
		stepNames[s.Name] = true

		if s.Workflow != "" {
			if !hasWorkflow(wf, s.Workflow) {
				return fmt.Errorf("workflow %q, step %q: references unknown workflow %q", workflowName, s.Name, s.Workflow)
			}
		} else if s.Type == "" {
			return fmt.Errorf("workflow %q, step %q: must have type or workflow", workflowName, s.Name)
		}

		if s.ForEach != "" && s.As == "" {
			return fmt.Errorf("workflow %q, step %q: for_each requires as", workflowName, s.Name)
		}

		if s.Type != "" {
			if err := resolveType(wf, s.Type); err != nil {
				return fmt.Errorf("workflow %q, step %q: %w", workflowName, s.Name, err)
			}
		}

		if s.Type == "gate" {
			if len(s.Rules) == 0 {
				return fmt.Errorf("workflow %q, step %q: gate must reference at least one rule", workflowName, s.Name)
			}
			for _, rn := range s.Rules {
				if wf.GetRule(rn) == nil {
					return fmt.Errorf("workflow %q, step %q: references unknown rule %q", workflowName, s.Name, rn)
				}
			}
		}
	}
	return nil
}

var primitives = map[string]bool{
	"k8s_job":       true,
	"http_request":  true,
	"llm_invoke":    true,
	"shell_exec":    true,
	"gate":          true,
	"load_artifact": true,
	"set_fact":      true,
	"assert":        true,
}

func resolveType(wf *WorkflowFile, typeName string) error {
	if primitives[typeName] {
		return nil
	}
	if _, ok := wf.StepTypes[typeName]; ok {
		return nil
	}
	return fmt.Errorf("unknown type %q (not a primitive or step_type)", typeName)
}

func hasWorkflow(wf *WorkflowFile, name string) bool {
	for _, w := range wf.Workflows {
		if w.Name == name {
			return true
		}
	}
	return false
}

func (wf *WorkflowFile) GetWorkflow(name string) *Workflow {
	for i := range wf.Workflows {
		if wf.Workflows[i].Name == name {
			return &wf.Workflows[i]
		}
	}
	return nil
}

func (wf *WorkflowFile) GetRule(name string) *Rule {
	for i := range wf.Rules {
		if wf.Rules[i].Name == name {
			return &wf.Rules[i]
		}
	}
	return nil
}

func loadRuleFiles(wf *WorkflowFile) error {
	var expanded []Rule
	for _, r := range wf.Rules {
		if r.File != "" {
			data, err := os.ReadFile(r.File)
			if err != nil {
				return fmt.Errorf("loading rule file %q: %w", r.File, err)
			}
			var rf struct {
				Rules []Rule `yaml:"rules"`
			}
			if err := yaml.Unmarshal(data, &rf); err != nil {
				return fmt.Errorf("parsing rule file %q: %w", r.File, err)
			}
			expanded = append(expanded, rf.Rules...)
		} else {
			expanded = append(expanded, r)
		}
	}
	wf.Rules = expanded
	return nil
}

func (wf *WorkflowFile) ResolveStepType(step *Step) (base string, mergedParams map[string]any) {
	if primitives[step.Type] {
		return step.Type, step.Params
	}
	st, ok := wf.StepTypes[step.Type]
	if !ok {
		return step.Type, step.Params
	}

	merged := make(map[string]any)
	for k, v := range st.Job {
		merged[k] = v
	}
	for k, v := range st.Defaults {
		merged[k] = v
	}
	for k, v := range st.Params {
		merged[k] = v
	}
	for k, v := range step.Params {
		merged[k] = v
	}

	return st.Base, merged
}
