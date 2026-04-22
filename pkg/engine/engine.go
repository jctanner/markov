package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jctanner/markov/pkg/executor"
	"github.com/jctanner/markov/pkg/parser"
	"github.com/jctanner/markov/pkg/state"
	"github.com/jctanner/markov/pkg/template"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Engine struct {
	file       *parser.WorkflowFile
	store      state.Store
	tmpl       *template.Engine
	executors  map[string]executor.Executor
	k8s        kubernetes.Interface
	restConfig *rest.Config
	forks      int
	Verbose    bool
}

func New(file *parser.WorkflowFile, store state.Store, executors map[string]executor.Executor) *Engine {
	forks := file.Forks
	if forks <= 0 {
		forks = 5
	}
	return &Engine{
		file:      file,
		store:     store,
		tmpl:      template.New(),
		executors: executors,
		forks:     forks,
	}
}

func (e *Engine) SetK8sClient(client kubernetes.Interface, cfg *rest.Config) {
	e.k8s = client
	e.restConfig = cfg
}

func (e *Engine) verbose(format string, args ...any) {
	if e.Verbose {
		log.Printf(format, args...)
	}
}

func (e *Engine) Run(ctx context.Context, workflowName string, vars map[string]any) (string, error) {
	if workflowName == "" {
		workflowName = e.file.Entrypoint
	}

	wf := e.file.GetWorkflow(workflowName)
	if wf == nil {
		return "", fmt.Errorf("workflow %q not found", workflowName)
	}

	runCtx := e.buildContext(vars, wf.Vars)

	runID := uuid.New().String()[:8]
	varsJSON, _ := json.Marshal(runCtx)

	run := &state.Run{
		RunID:        runID,
		WorkflowFile: "",
		Entrypoint:   workflowName,
		Status:       state.RunRunning,
		VarsJSON:     string(varsJSON),
		StartedAt:    time.Now(),
	}
	if err := e.store.CreateRun(ctx, run); err != nil {
		return "", fmt.Errorf("creating run: %w", err)
	}

	log.Printf("[run:%s] starting workflow %q", runID, workflowName)
	e.verbose("[run:%s]   forks: %d", runID, e.forks)
	e.verbose("[run:%s]   namespace: %s", runID, e.file.Namespace)
	e.verbose("[run:%s]   vars: %s", runID, string(varsJSON))
	e.verbose("[run:%s]   steps: %d", runID, len(wf.Steps))

	err := e.executeWorkflow(ctx, runID, wf, runCtx)

	now := time.Now()
	run.CompletedAt = &now
	if err != nil {
		run.Status = state.RunFailed
		log.Printf("[run:%s] workflow %q failed: %v", runID, workflowName, err)
	} else {
		run.Status = state.RunCompleted
		log.Printf("[run:%s] workflow %q completed", runID, workflowName)
	}
	e.store.UpdateRun(ctx, run)

	return runID, err
}

func (e *Engine) Resume(ctx context.Context, runID string) error {
	run, err := e.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}

	wf := e.file.GetWorkflow(run.Entrypoint)
	if wf == nil {
		return fmt.Errorf("workflow %q not found", run.Entrypoint)
	}

	var runCtx map[string]any
	json.Unmarshal([]byte(run.VarsJSON), &runCtx)

	steps, err := e.store.GetSteps(ctx, runID)
	if err != nil {
		return err
	}
	for _, s := range steps {
		if s.Status == state.StepCompleted {
			if isSetFactStep(wf, s.StepName) && s.OutputJSON != "" {
				var facts map[string]any
				json.Unmarshal([]byte(s.OutputJSON), &facts)
				for k, v := range facts {
					runCtx[k] = v
				}
			} else if s.ArtifactsJSON != "" {
				var stepData map[string]any
				json.Unmarshal([]byte(s.ArtifactsJSON), &stepData)
				runCtx[s.StepName] = stepData
			} else if s.OutputJSON != "" {
				var output map[string]any
				json.Unmarshal([]byte(s.OutputJSON), &output)
				runCtx[s.StepName] = output
			}
		}
	}

	run.Status = state.RunRunning
	e.store.UpdateRun(ctx, run)

	log.Printf("[run:%s] resuming workflow %q", runID, run.Entrypoint)

	err = e.executeWorkflow(ctx, runID, wf, runCtx)

	now := time.Now()
	run.CompletedAt = &now
	if err != nil {
		run.Status = state.RunFailed
	} else {
		run.Status = state.RunCompleted
	}
	e.store.UpdateRun(ctx, run)

	return err
}

func (e *Engine) executeWorkflow(ctx context.Context, runID string, wf *parser.Workflow, runCtx map[string]any) error {
	for _, step := range wf.Steps {
		if err := e.executeStep(ctx, runID, wf.Name, step, runCtx); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}
	}
	return nil
}

func (e *Engine) executeStep(ctx context.Context, runID string, workflowName string, step parser.Step, runCtx map[string]any) error {
	existing, _ := e.store.GetStep(ctx, runID, workflowName, step.Name)
	if existing != nil && existing.Status == state.StepCompleted {
		log.Printf("[run:%s] skipping completed step %q", runID, step.Name)
		if existing.OutputJSON != "" {
			var output map[string]any
			json.Unmarshal([]byte(existing.OutputJSON), &output)
			if step.Type == "set_fact" {
				for k, v := range output {
					runCtx[k] = v
				}
			} else if step.Register != "" {
				runCtx[step.Register] = output
			}
		}
		return nil
	}

	if step.When != "" {
		ok, err := e.tmpl.EvalBool(step.When, runCtx)
		if err != nil {
			return fmt.Errorf("evaluating when: %w", err)
		}
		if !ok {
			log.Printf("[run:%s] skipping step %q (when: %q is false)", runID, step.Name, step.When)
			now := time.Now()
			e.store.SaveStep(ctx, &state.StepResult{
				RunID:        runID,
				WorkflowName: workflowName,
				StepName:     step.Name,
				Status:       state.StepSkipped,
				StartedAt:    &now,
				CompletedAt:  &now,
			})
			return nil
		}
	}

	if step.ForEach != "" {
		return e.executeForEach(ctx, runID, workflowName, step, runCtx)
	}

	if step.Workflow != "" {
		return e.executeSubWorkflow(ctx, runID, workflowName, step, runCtx)
	}

	now := time.Now()

	base, mergedParams := e.file.ResolveStepType(&step)

	if base == "set_fact" {
		log.Printf("[run:%s] setting facts for step %q", runID, step.Name)
		if len(step.Vars) == 0 {
			return e.failStep(ctx, runID, workflowName, step.Name, fmt.Errorf("set_fact step has no vars defined"))
		}
		facts, err := e.evalFacts(step.Vars, runCtx)
		if err != nil {
			return e.failStep(ctx, runID, workflowName, step.Name, fmt.Errorf("evaluating facts: %w", err))
		}
		for k, v := range facts {
			runCtx[k] = v
			e.verbose("[run:%s]   %s = %v", runID, k, v)
		}
		factsJSON, _ := json.Marshal(facts)
		completed := time.Now()
		e.store.SaveStep(ctx, &state.StepResult{
			RunID:        runID,
			WorkflowName: workflowName,
			StepName:     step.Name,
			Status:       state.StepCompleted,
			OutputJSON:   string(factsJSON),
			StartedAt:    &now,
			CompletedAt:  &completed,
		})
		log.Printf("[run:%s] step %q completed", runID, step.Name)
		return nil
	}

	if base == "load_artifact" {
		log.Printf("[run:%s] loading artifacts for step %q", runID, step.Name)
		if len(step.Artifacts) == 0 {
			return e.failStep(ctx, runID, workflowName, step.Name, fmt.Errorf("load_artifact step has no artifacts defined"))
		}
		artifacts, err := e.loadArtifacts(step.Artifacts, runCtx)
		if err != nil {
			return e.failStep(ctx, runID, workflowName, step.Name, fmt.Errorf("loading artifacts: %w", err))
		}
		runCtx[step.Name] = map[string]any{"artifacts": artifacts}
		e.verbose("[run:%s]   loaded %d artifact(s)", runID, len(artifacts))
		completed := time.Now()
		e.store.SaveStep(ctx, &state.StepResult{
			RunID:        runID,
			WorkflowName: workflowName,
			StepName:     step.Name,
			Status:       state.StepCompleted,
			StartedAt:    &now,
			CompletedAt:  &completed,
		})
		log.Printf("[run:%s] step %q completed", runID, step.Name)
		return nil
	}

	e.store.SaveStep(ctx, &state.StepResult{
		RunID:        runID,
		WorkflowName: workflowName,
		StepName:     step.Name,
		Status:       state.StepRunning,
		StartedAt:    &now,
	})

	log.Printf("[run:%s] executing step %q (type: %s)", runID, step.Name, step.Type)
	e.verbose("[run:%s]   resolved type: %s -> %s", runID, step.Type, base)

	renderedParams, err := e.tmpl.RenderMap(mergedParams, runCtx)
	if err != nil {
		return e.failStep(ctx, runID, workflowName, step.Name, fmt.Errorf("rendering params: %w", err))
	}

	if base == "k8s_job" {
		e.injectJobMeta(renderedParams, runID, workflowName, step.Name)
		e.verbose("[run:%s]   job name: %s", runID, renderedParams["_job_name"])
		e.verbose("[run:%s]   image: %s", runID, renderedParams["image"])
		if cmd, ok := renderedParams["command"]; ok {
			e.verbose("[run:%s]   command: %v", runID, cmd)
		}
		if args, ok := renderedParams["args"]; ok {
			e.verbose("[run:%s]   args: %v", runID, args)
		}
		if ns, ok := renderedParams["namespace"]; ok {
			e.verbose("[run:%s]   namespace: %s", runID, ns)
		}
	} else {
		for k, v := range renderedParams {
			e.verbose("[run:%s]   param %s: %v", runID, k, v)
		}
	}

	exec, ok := e.executors[base]
	if !ok {
		return e.failStep(ctx, runID, workflowName, step.Name, fmt.Errorf("no executor for type %q", base))
	}

	var execCtx context.Context
	var cancel context.CancelFunc
	if step.Timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		e.verbose("[run:%s]   timeout: %ds", runID, step.Timeout)
	} else {
		execCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	result, err := exec.Execute(execCtx, renderedParams)
	if err != nil {
		return e.failStep(ctx, runID, workflowName, step.Name, err)
	}

	output := result.Output
	if step.Register != "" {
		runCtx[step.Register] = output
		e.verbose("[run:%s]   registered %q: %v", runID, step.Register, output)
	}

	if len(step.Artifacts) > 0 {
		artifacts, err := e.loadArtifacts(step.Artifacts, runCtx)
		if err != nil {
			return e.failStep(ctx, runID, workflowName, step.Name, fmt.Errorf("loading artifacts: %w", err))
		}
		stepData := map[string]any{"artifacts": artifacts}
		if output != nil {
			for k, v := range output {
				stepData[k] = v
			}
		}
		runCtx[step.Name] = stepData
		e.verbose("[run:%s]   loaded %d artifact(s) for %q", runID, len(artifacts), step.Name)
	}

	outputJSON, _ := json.Marshal(output)
	artifactsJSON := ""
	if stepData, ok := runCtx[step.Name]; ok {
		aj, _ := json.Marshal(stepData)
		artifactsJSON = string(aj)
	}
	completed := time.Now()
	e.store.SaveStep(ctx, &state.StepResult{
		RunID:         runID,
		WorkflowName:  workflowName,
		StepName:      step.Name,
		Status:        state.StepCompleted,
		OutputJSON:    string(outputJSON),
		ArtifactsJSON: artifactsJSON,
		StartedAt:     &now,
		CompletedAt:   &completed,
	})

	log.Printf("[run:%s] step %q completed", runID, step.Name)
	return nil
}

func (e *Engine) executeSubWorkflow(ctx context.Context, runID string, workflowName string, step parser.Step, runCtx map[string]any) error {
	wf := e.file.GetWorkflow(step.Workflow)
	if wf == nil {
		return fmt.Errorf("sub-workflow %q not found", step.Workflow)
	}

	subVars := make(map[string]any)
	for k, v := range runCtx {
		subVars[k] = v
	}
	for k, v := range wf.Vars {
		if v != nil {
			subVars[k] = v
		}
	}
	if step.Vars != nil {
		rendered, err := e.tmpl.RenderMap(step.Vars, runCtx)
		if err != nil {
			return fmt.Errorf("rendering sub-workflow vars: %w", err)
		}
		for k, v := range rendered {
			subVars[k] = v
		}
	}

	subRunID := fmt.Sprintf("%s-%s", runID, step.Name)
	varsJSON, _ := json.Marshal(subVars)
	subRun := &state.Run{
		RunID:       subRunID,
		Entrypoint:  step.Workflow,
		Status:      state.RunRunning,
		VarsJSON:    string(varsJSON),
		ParentRunID: runID,
		ParentStep:  step.Name,
		StartedAt:   time.Now(),
	}
	e.store.CreateRun(ctx, subRun)

	log.Printf("[run:%s] starting sub-workflow %q as %s", runID, step.Workflow, subRunID)

	err := e.executeWorkflow(ctx, subRunID, wf, subVars)

	now := time.Now()
	subRun.CompletedAt = &now
	if err != nil {
		subRun.Status = state.RunFailed
		e.store.UpdateRun(ctx, subRun)
		return err
	}
	subRun.Status = state.RunCompleted
	e.store.UpdateRun(ctx, subRun)

	if step.Register != "" {
		runCtx[step.Register] = subVars
	}

	log.Printf("[run:%s] sub-workflow %q completed", runID, step.Workflow)
	return nil
}

func (e *Engine) executeForEach(ctx context.Context, runID string, workflowName string, step parser.Step, runCtx map[string]any) error {
	listVal, err := e.resolveForEachList(step.ForEach, runCtx)
	if err != nil {
		return fmt.Errorf("resolving for_each: %w", err)
	}

	concurrency := step.Concurrency
	if concurrency <= 0 {
		concurrency = e.forks
	}

	log.Printf("[run:%s] for_each %q: %d items, concurrency %d", runID, step.Name, len(listVal), concurrency)

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var results []map[string]any
	var firstErr error
	var errOnce sync.Once

	var wg sync.WaitGroup

	for i, item := range listVal {
		if firstErr != nil {
			break
		}

		sem <- struct{}{}
		wg.Add(1)

		go func(idx int, itemVal any) {
			defer wg.Done()
			defer func() { <-sem }()

			itemCtx := make(map[string]any)
			for k, v := range runCtx {
				itemCtx[k] = v
			}
			itemCtx[step.As] = itemVal

			if step.Workflow != "" {
				wf := e.file.GetWorkflow(step.Workflow)
				if wf == nil {
					errOnce.Do(func() { firstErr = fmt.Errorf("sub-workflow %q not found", step.Workflow) })
					return
				}

				subVars := make(map[string]any)
				for k, v := range runCtx {
					subVars[k] = v
				}
				for k, v := range wf.Vars {
					subVars[k] = v
				}
				if step.Vars != nil {
					rendered, err := e.tmpl.RenderMap(step.Vars, itemCtx)
					if err != nil {
						errOnce.Do(func() { firstErr = fmt.Errorf("rendering sub-workflow vars: %w", err) })
						return
					}
					for k, v := range rendered {
						subVars[k] = v
					}
				}

				subRunID := fmt.Sprintf("%s-%s-%d", runID, step.Name, idx)
				varsJSON, _ := json.Marshal(subVars)
				subRun := &state.Run{
					RunID:       subRunID,
					Entrypoint:  step.Workflow,
					Status:      state.RunRunning,
					VarsJSON:    string(varsJSON),
					ParentRunID: runID,
					ParentStep:  step.Name,
					ForEachKey:  fmt.Sprintf("%d", idx),
					StartedAt:   time.Now(),
				}
				e.store.CreateRun(ctx, subRun)

				err := e.executeWorkflow(ctx, subRunID, wf, subVars)

				now := time.Now()
				subRun.CompletedAt = &now
				if err != nil {
					subRun.Status = state.RunFailed
					errOnce.Do(func() { firstErr = err })
				} else {
					subRun.Status = state.RunCompleted
				}
				e.store.UpdateRun(ctx, subRun)

				mu.Lock()
				results = append(results, subVars)
				mu.Unlock()
			} else {
				subStep := step
				subStep.ForEach = ""
				subStep.Register = ""

				err := e.executeStep(ctx, runID, workflowName, subStep, itemCtx)
				if err != nil {
					errOnce.Do(func() { firstErr = err })
					return
				}

				mu.Lock()
				results = append(results, itemCtx)
				mu.Unlock()
			}
		}(i, item)
	}

	wg.Wait()

	if step.Register != "" {
		runCtx[step.Register] = results
	}

	if firstErr != nil {
		return firstErr
	}

	return nil
}

func (e *Engine) resolveForEachList(expr string, ctx map[string]any) ([]any, error) {
	val := resolveContextPath(expr, ctx)
	if val != nil {
		switch v := val.(type) {
		case []any:
			return v, nil
		case []string:
			list := make([]any, len(v))
			for i, s := range v {
				list[i] = s
			}
			return list, nil
		}
	}

	rendered, err := e.tmpl.Render("{{ "+expr+" }}", ctx)
	if err != nil {
		return nil, err
	}

	var list []any
	if err := json.Unmarshal([]byte(rendered), &list); err != nil {
		return nil, fmt.Errorf("for_each expression %q did not resolve to a list", expr)
	}
	return list, nil
}

func resolveContextPath(path string, ctx map[string]any) any {
	parts := strings.Split(path, ".")
	var current any = ctx
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

func (e *Engine) injectJobMeta(params map[string]any, runID, workflowName, stepName string) {
	if _, ok := params["_job_name"]; !ok {
		prefix := "markov"
		if p, ok := params["name_prefix"].(string); ok && p != "" {
			prefix = p
		}
		sanitized := strings.NewReplacer("_", "-", " ", "-").Replace(strings.ToLower(stepName))
		params["_job_name"] = fmt.Sprintf("%s-%s-%s", prefix, runID, sanitized)
	}

	labels, _ := params["_labels"].(map[string]string)
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app"] = "markov"
	labels["markov/run-id"] = runID
	labels["markov/workflow"] = workflowName
	labels["markov/step"] = stepName
	params["_labels"] = labels
}

func (e *Engine) failStep(ctx context.Context, runID, workflowName, stepName string, err error) error {
	now := time.Now()
	e.store.SaveStep(ctx, &state.StepResult{
		RunID:        runID,
		WorkflowName: workflowName,
		StepName:     stepName,
		Status:       state.StepFailed,
		Error:        err.Error(),
		CompletedAt:  &now,
	})
	return err
}

func isSetFactStep(wf *parser.Workflow, stepName string) bool {
	for _, s := range wf.Steps {
		if s.Name == stepName {
			return s.Type == "set_fact"
		}
	}
	return false
}

func (e *Engine) buildContext(cliVars map[string]any, workflowVars map[string]any) map[string]any {
	ctx := make(map[string]any)
	for k, v := range e.file.Vars {
		ctx[k] = v
	}
	for k, v := range workflowVars {
		if v != nil {
			ctx[k] = v
		}
	}
	for k, v := range cliVars {
		ctx[k] = v
	}
	return ctx
}
