package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jctanner/markov/pkg/callback"
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
	RunID      string
	callbacks  []callback.Callback
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

func (e *Engine) SetCallbacks(cbs []callback.Callback) {
	e.callbacks = cbs
}

func (e *Engine) CloseCallbacks() {
	for _, cb := range e.callbacks {
		if err := cb.Close(); err != nil {
			log.Printf("callback close error: %v", err)
		}
	}
}

func (e *Engine) fireEvent(fn func(callback.Callback) error) {
	e.verbose("firing callback event to %d callback(s)", len(e.callbacks))
	for _, cb := range e.callbacks {
		if err := fn(cb); err != nil {
			log.Printf("callback error: %v", err)
		}
	}
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

	runID := e.RunID
	if runID == "" {
		runID = uuid.New().String()[:8]
	}
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

	e.fireEvent(func(cb callback.Callback) error {
		return cb.OnRunStarted(callback.RunStartedEvent{
			EventHeader:  callback.EventHeader{Timestamp: run.StartedAt, RunID: runID, EventType: "run_started"},
			WorkflowName: workflowName,
			Vars:         runCtx,
			Forks:        e.forks,
			Namespace:    e.file.Namespace,
		})
	})

	log.Printf("[run:%s] starting workflow %q", runID, workflowName)
	e.verbose("[run:%s]   forks: %d", runID, e.forks)
	e.verbose("[run:%s]   namespace: %s", runID, e.file.Namespace)
	e.verbose("[run:%s]   vars: %s", runID, string(varsJSON))
	e.verbose("[run:%s]   steps: %d", runID, len(wf.Steps))

	err := e.executeWorkflow(ctx, runID, wf, runCtx)

	now := time.Now()
	run.CompletedAt = &now
	duration := now.Sub(run.StartedAt).Seconds()
	if err != nil {
		run.Status = state.RunFailed
		log.Printf("[run:%s] workflow %q failed: %v", runID, workflowName, err)
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnRunFailed(callback.RunFailedEvent{
				EventHeader:  callback.EventHeader{Timestamp: now, RunID: runID, EventType: "run_failed"},
				WorkflowName: workflowName,
				Error:        err.Error(),
				Duration:     duration,
			})
		})
	} else {
		run.Status = state.RunCompleted
		log.Printf("[run:%s] workflow %q completed", runID, workflowName)
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnRunCompleted(callback.RunCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: now, RunID: runID, EventType: "run_completed"},
				WorkflowName: workflowName,
				Duration:     duration,
			})
		})
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
			} else if isGateStep(wf, s.StepName) && s.OutputJSON != "" {
				var output map[string]any
				json.Unmarshal([]byte(s.OutputJSON), &output)
				if facts, ok := output["facts"].(map[string]any); ok {
					for k, v := range facts {
						runCtx[k] = v
					}
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

	completedCount := 0
	for _, s := range steps {
		if s.Status == state.StepCompleted {
			completedCount++
		}
	}
	e.fireEvent(func(cb callback.Callback) error {
		return cb.OnRunResumed(callback.RunResumedEvent{
			EventHeader:    callback.EventHeader{Timestamp: time.Now(), RunID: runID, EventType: "run_resumed"},
			WorkflowName:   run.Entrypoint,
			CompletedSteps: completedCount,
			RemainingSteps: len(wf.Steps) - completedCount,
		})
	})

	log.Printf("[run:%s] resuming workflow %q", runID, run.Entrypoint)

	resumeStart := time.Now()
	err = e.executeWorkflow(ctx, runID, wf, runCtx)

	now := time.Now()
	run.CompletedAt = &now
	duration := now.Sub(resumeStart).Seconds()
	if err != nil {
		run.Status = state.RunFailed
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnRunFailed(callback.RunFailedEvent{
				EventHeader:  callback.EventHeader{Timestamp: now, RunID: runID, EventType: "run_failed"},
				WorkflowName: run.Entrypoint,
				Error:        err.Error(),
				Duration:     duration,
			})
		})
	} else {
		run.Status = state.RunCompleted
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnRunCompleted(callback.RunCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: now, RunID: runID, EventType: "run_completed"},
				WorkflowName: run.Entrypoint,
				Duration:     duration,
			})
		})
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
			} else if step.Type == "gate" {
				if facts, ok := output["facts"].(map[string]any); ok {
					for k, v := range facts {
						runCtx[k] = v
					}
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
			e.fireEvent(func(cb callback.Callback) error {
				return cb.OnStepSkipped(callback.StepSkippedEvent{
					EventHeader:  callback.EventHeader{Timestamp: now, RunID: runID, EventType: "step_skipped"},
					WorkflowName: workflowName,
					StepName:     step.Name,
					Reason:       fmt.Sprintf("when condition %q evaluated to false", step.When),
				})
			})
			return nil
		}
	}

	if step.ForEach != "" {
		feStart := time.Now()
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepStarted(callback.StepStartedEvent{
				EventHeader:  callback.EventHeader{Timestamp: feStart, RunID: runID, EventType: "step_started"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				ResolvedType: "for_each",
			})
		})
		err := e.executeForEach(ctx, runID, workflowName, step, runCtx)
		feEnd := time.Now()
		if err != nil {
			e.fireEvent(func(cb callback.Callback) error {
				return cb.OnStepFailed(callback.StepFailedEvent{
					EventHeader:  callback.EventHeader{Timestamp: feEnd, RunID: runID, EventType: "step_failed"},
					WorkflowName: workflowName,
					StepName:     step.Name,
					StepType:     step.Type,
					Error:        err.Error(),
					Duration:     feEnd.Sub(feStart).Seconds(),
				})
			})
			return err
		}
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepCompleted(callback.StepCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: feEnd, RunID: runID, EventType: "step_completed"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				Duration:     feEnd.Sub(feStart).Seconds(),
			})
		})
		return nil
	}

	if step.Workflow != "" {
		swStart := time.Now()
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepStarted(callback.StepStartedEvent{
				EventHeader:  callback.EventHeader{Timestamp: swStart, RunID: runID, EventType: "step_started"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				ResolvedType: "workflow",
			})
		})
		err := e.executeSubWorkflow(ctx, runID, workflowName, step, runCtx)
		swEnd := time.Now()
		if err != nil {
			e.fireEvent(func(cb callback.Callback) error {
				return cb.OnStepFailed(callback.StepFailedEvent{
					EventHeader:  callback.EventHeader{Timestamp: swEnd, RunID: runID, EventType: "step_failed"},
					WorkflowName: workflowName,
					StepName:     step.Name,
					StepType:     step.Type,
					Error:        err.Error(),
					Duration:     swEnd.Sub(swStart).Seconds(),
				})
			})
			return err
		}
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepCompleted(callback.StepCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: swEnd, RunID: runID, EventType: "step_completed"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				Duration:     swEnd.Sub(swStart).Seconds(),
			})
		})
		return nil
	}

	now := time.Now()

	base, mergedParams := e.file.ResolveStepType(&step)

	e.fireEvent(func(cb callback.Callback) error {
		return cb.OnStepStarted(callback.StepStartedEvent{
			EventHeader:  callback.EventHeader{Timestamp: now, RunID: runID, EventType: "step_started"},
			WorkflowName: workflowName,
			StepName:     step.Name,
			StepType:     step.Type,
			ResolvedType: base,
			Params:       mergedParams,
		})
	})

	if base == "set_fact" {
		log.Printf("[run:%s] setting facts for step %q", runID, step.Name)
		if len(step.Vars) == 0 {
			return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("set_fact step has no vars defined"))
		}
		facts, err := e.evalFacts(step.Vars, runCtx)
		if err != nil {
			return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("evaluating facts: %w", err))
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
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepCompleted(callback.StepCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: completed, RunID: runID, EventType: "step_completed"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				Output:       facts,
				Duration:     completed.Sub(now).Seconds(),
			})
		})
		log.Printf("[run:%s] step %q completed", runID, step.Name)
		return nil
	}

	if base == "assert" {
		log.Printf("[run:%s] evaluating assertions for step %q", runID, step.Name)
		if len(step.That) == 0 {
			return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("assert step has no conditions defined"))
		}
		for _, expr := range step.That {
			ok, err := e.tmpl.EvalBool(expr, runCtx)
			if err != nil {
				return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("evaluating assertion %q: %w", expr, err))
			}
			if !ok {
				msg := step.Msg
				if msg == "" {
					msg = fmt.Sprintf("assertion failed: %s", expr)
				}
				return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("%s", msg))
			}
			e.verbose("[run:%s]   assert %q: ok", runID, expr)
		}
		completed := time.Now()
		e.store.SaveStep(ctx, &state.StepResult{
			RunID:        runID,
			WorkflowName: workflowName,
			StepName:     step.Name,
			Status:       state.StepCompleted,
			StartedAt:    &now,
			CompletedAt:  &completed,
		})
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepCompleted(callback.StepCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: completed, RunID: runID, EventType: "step_completed"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				Duration:     completed.Sub(now).Seconds(),
			})
		})
		log.Printf("[run:%s] step %q completed", runID, step.Name)
		return nil
	}

	if base == "gate" {
		log.Printf("[run:%s] evaluating gate %q (%d rules)", runID, step.Name, len(step.Rules))
		result, err := e.evaluateGate(runID, step.Rules, step.Facts, runCtx)
		if err != nil {
			return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("gate evaluation: %w", err))
		}

		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnGateEvaluated(callback.GateEvaluatedEvent{
				EventHeader:  callback.EventHeader{Timestamp: time.Now(), RunID: runID, EventType: "gate_evaluated"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				Action:       result.Action,
				FiredRules:   result.FiredRules,
				Facts:        result.Facts,
			})
		})

		log.Printf("[run:%s] gate %q: action=%s, fired=%v", runID, step.Name, result.Action, result.FiredRules)

		if result.Action == "pause" {
			log.Printf("[run:%s] gate %q requested pause (not yet implemented, continuing)", runID, step.Name)
		}

		outputJSON, _ := json.Marshal(map[string]any{
			"action":      result.Action,
			"fired_rules": result.FiredRules,
			"facts":       result.Facts,
		})
		completed := time.Now()
		e.store.SaveStep(ctx, &state.StepResult{
			RunID:        runID,
			WorkflowName: workflowName,
			StepName:     step.Name,
			Status:       state.StepCompleted,
			OutputJSON:   string(outputJSON),
			StartedAt:    &now,
			CompletedAt:  &completed,
		})
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepCompleted(callback.StepCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: completed, RunID: runID, EventType: "step_completed"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				Duration:     completed.Sub(now).Seconds(),
			})
		})
		log.Printf("[run:%s] step %q completed", runID, step.Name)
		return nil
	}

	if base == "load_artifact" {
		log.Printf("[run:%s] loading artifacts for step %q", runID, step.Name)
		if len(step.Artifacts) == 0 {
			return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("load_artifact step has no artifacts defined"))
		}
		artifacts, err := e.loadArtifacts(step.Artifacts, runCtx)
		if err != nil {
			return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("loading artifacts: %w", err))
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
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnStepCompleted(callback.StepCompletedEvent{
				EventHeader:  callback.EventHeader{Timestamp: completed, RunID: runID, EventType: "step_completed"},
				WorkflowName: workflowName,
				StepName:     step.Name,
				StepType:     step.Type,
				Duration:     completed.Sub(now).Seconds(),
			})
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
		return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("rendering params: %w", err))
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
		return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("no executor for type %q", base))
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

	if k8sExec, ok := exec.(*executor.K8sJob); ok {
		stepName := step.Name
		stepType := step.Type
		k8sExec.SetOnJobCreated(func(info executor.K8sJobInfo) {
			e.fireEvent(func(cb callback.Callback) error {
				return cb.OnJobCreated(callback.JobCreatedEvent{
					EventHeader:  callback.EventHeader{Timestamp: time.Now(), RunID: runID, EventType: "job_created"},
					WorkflowName: workflowName,
					StepName:     stepName,
					StepType:     stepType,
					JobName:      info.JobName,
					Namespace:    info.Namespace,
					PodSelector:  fmt.Sprintf("job-name=%s", info.JobName),
				})
			})
		})
		defer k8sExec.SetOnJobCreated(nil)
	}

	result, err := exec.Execute(execCtx, renderedParams)
	if err != nil {
		return e.failStep(ctx, runID, workflowName, step.Name, base, now, err)
	}

	output := result.Output
	if step.Register != "" {
		runCtx[step.Register] = output
		e.verbose("[run:%s]   registered %q: %v", runID, step.Register, output)
	}

	if len(step.Artifacts) > 0 {
		artifacts, err := e.loadArtifacts(step.Artifacts, runCtx)
		if err != nil {
			return e.failStep(ctx, runID, workflowName, step.Name, base, now, fmt.Errorf("loading artifacts: %w", err))
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

	e.fireEvent(func(cb callback.Callback) error {
		return cb.OnStepCompleted(callback.StepCompletedEvent{
			EventHeader:  callback.EventHeader{Timestamp: completed, RunID: runID, EventType: "step_completed"},
			WorkflowName: workflowName,
			StepName:     step.Name,
			StepType:     step.Type,
			Output:       output,
			Duration:     completed.Sub(now).Seconds(),
		})
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
			if s, ok := v.(string); ok {
				subVars[k] = coerceString(s)
			} else {
				subVars[k] = v
			}
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

	e.fireEvent(func(cb callback.Callback) error {
		return cb.OnSubRunStarted(callback.SubRunStartedEvent{
			EventHeader:  callback.EventHeader{Timestamp: subRun.StartedAt, RunID: subRunID, EventType: "sub_run_started"},
			ParentRunID:  runID,
			ParentStep:   step.Name,
			WorkflowName: step.Workflow,
		})
	})

	log.Printf("[run:%s] starting sub-workflow %q as %s", runID, step.Workflow, subRunID)

	err := e.executeWorkflow(ctx, subRunID, wf, subVars)

	subNow := time.Now()
	subRun.CompletedAt = &subNow
	duration := subNow.Sub(subRun.StartedAt).Seconds()
	if err != nil {
		subRun.Status = state.RunFailed
		e.store.UpdateRun(ctx, subRun)
		e.fireEvent(func(cb callback.Callback) error {
			return cb.OnSubRunFailed(callback.SubRunFailedEvent{
				EventHeader:  callback.EventHeader{Timestamp: subNow, RunID: subRunID, EventType: "sub_run_failed"},
				ParentRunID:  runID,
				ParentStep:   step.Name,
				WorkflowName: step.Workflow,
				Error:        err.Error(),
				Duration:     duration,
			})
		})
		return err
	}
	subRun.Status = state.RunCompleted
	e.store.UpdateRun(ctx, subRun)

	e.fireEvent(func(cb callback.Callback) error {
		return cb.OnSubRunCompleted(callback.SubRunCompletedEvent{
			EventHeader:  callback.EventHeader{Timestamp: subNow, RunID: subRunID, EventType: "sub_run_completed"},
			ParentRunID:  runID,
			ParentStep:   step.Name,
			WorkflowName: step.Workflow,
			Duration:     duration,
		})
	})

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

	if step.ForEachSort != "" {
		for i, item := range listVal {
			if extractItemField(item, step.ForEachSort) == nil {
				return fmt.Errorf("for_each_sort %q not found on item at index %d", step.ForEachSort, i)
			}
		}
		sort.SliceStable(listVal, func(i, j int) bool {
			ki := fmt.Sprintf("%v", extractItemField(listVal[i], step.ForEachSort))
			kj := fmt.Sprintf("%v", extractItemField(listVal[j], step.ForEachSort))
			return ki < kj
		})
	}

	if step.ForEachKey != "" {
		seen := make(map[string]int)
		for i, item := range listVal {
			val := extractItemField(item, step.ForEachKey)
			if val == nil {
				return fmt.Errorf("for_each_key %q not found on item at index %d", step.ForEachKey, i)
			}
			key := fmt.Sprintf("%v", val)
			if prev, ok := seen[key]; ok {
				return fmt.Errorf("duplicate for_each_key %q at indices %d and %d", key, prev, i)
			}
			seen[key] = i
		}
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
						if s, ok := v.(string); ok {
							subVars[k] = coerceString(s)
						} else {
							subVars[k] = v
						}
					}
				}

				forEachKey := fmt.Sprintf("%d", idx)
				if step.ForEachKey != "" {
					forEachKey = fmt.Sprintf("%v", extractItemField(itemVal, step.ForEachKey))
				}

				subRunID := fmt.Sprintf("%s-%s-%s", runID, step.Name, forEachKey)
				varsJSON, _ := json.Marshal(subVars)
				subRun := &state.Run{
					RunID:       subRunID,
					Entrypoint:  step.Workflow,
					Status:      state.RunRunning,
					VarsJSON:    string(varsJSON),
					ParentRunID: runID,
					ParentStep:  step.Name,
					ForEachKey:  forEachKey,
					StartedAt:   time.Now(),
				}
				e.store.CreateRun(ctx, subRun)
				e.fireEvent(func(cb callback.Callback) error {
					return cb.OnSubRunStarted(callback.SubRunStartedEvent{
						EventHeader:  callback.EventHeader{Timestamp: subRun.StartedAt, RunID: subRunID, EventType: "sub_run_started"},
						ParentRunID:  runID,
						ParentStep:   step.Name,
						WorkflowName: step.Workflow,
						ForEachKey:   forEachKey,
					})
				})

				err := e.executeWorkflow(ctx, subRunID, wf, subVars)

				feNow := time.Now()
				subRun.CompletedAt = &feNow
				feDuration := feNow.Sub(subRun.StartedAt).Seconds()
				if err != nil {
					subRun.Status = state.RunFailed
					errOnce.Do(func() { firstErr = err })
					e.fireEvent(func(cb callback.Callback) error {
						return cb.OnSubRunFailed(callback.SubRunFailedEvent{
							EventHeader:  callback.EventHeader{Timestamp: feNow, RunID: subRunID, EventType: "sub_run_failed"},
							ParentRunID:  runID,
							ParentStep:   step.Name,
							WorkflowName: step.Workflow,
							ForEachKey:   forEachKey,
							Error:        err.Error(),
							Duration:     feDuration,
						})
					})
				} else {
					subRun.Status = state.RunCompleted
					e.fireEvent(func(cb callback.Callback) error {
						return cb.OnSubRunCompleted(callback.SubRunCompletedEvent{
							EventHeader:  callback.EventHeader{Timestamp: feNow, RunID: subRunID, EventType: "sub_run_completed"},
							ParentRunID:  runID,
							ParentStep:   step.Name,
							WorkflowName: step.Workflow,
							ForEachKey:   forEachKey,
							Duration:     feDuration,
						})
					})
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

func extractItemField(item any, field string) any {
	m, ok := item.(map[string]any)
	if !ok {
		return nil
	}
	v, ok := m[field]
	if !ok {
		return nil
	}
	return v
}

var k8sNameUnsafe = regexp.MustCompile(`[^a-z0-9\-.]`)

func sanitizeK8sName(s string, maxLen int) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("_", "-", " ", "-").Replace(s)
	s = k8sNameUnsafe.ReplaceAllString(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-.")
	if len(s) > maxLen {
		hash := sha256.Sum256([]byte(s))
		suffix := hex.EncodeToString(hash[:4])
		s = s[:maxLen-len(suffix)-1] + "-" + suffix
		s = strings.TrimRight(s, "-.")
	}
	return s
}

func sanitizeK8sLabel(s string, maxLen int) string {
	s = strings.NewReplacer("_", "-", " ", "-").Replace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	s = strings.Trim(s, "-._")
	return s
}

func (e *Engine) injectJobMeta(params map[string]any, runID, workflowName, stepName string) {
	if _, ok := params["_job_name"]; !ok {
		prefix := "markov"
		if p, ok := params["name_prefix"].(string); ok && p != "" {
			prefix = p
		}
		raw := fmt.Sprintf("%s-%s-%s", prefix, runID, stepName)
		params["_job_name"] = sanitizeK8sName(raw, 63)
	}

	labels, _ := params["_labels"].(map[string]string)
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["app"] = "markov"
	labels["markov/run-id"] = sanitizeK8sLabel(runID, 63)
	labels["markov/workflow"] = sanitizeK8sLabel(workflowName, 63)
	labels["markov/step"] = sanitizeK8sLabel(stepName, 63)
	params["_labels"] = labels
}

func (e *Engine) failStep(ctx context.Context, runID, workflowName, stepName, stepType string, startedAt time.Time, err error) error {
	now := time.Now()
	e.store.SaveStep(ctx, &state.StepResult{
		RunID:        runID,
		WorkflowName: workflowName,
		StepName:     stepName,
		Status:       state.StepFailed,
		Error:        err.Error(),
		StartedAt:    &startedAt,
		CompletedAt:  &now,
	})
	e.fireEvent(func(cb callback.Callback) error {
		return cb.OnStepFailed(callback.StepFailedEvent{
			EventHeader:  callback.EventHeader{Timestamp: now, RunID: runID, EventType: "step_failed"},
			WorkflowName: workflowName,
			StepName:     stepName,
			StepType:     stepType,
			Error:        err.Error(),
			Duration:     now.Sub(startedAt).Seconds(),
		})
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

func isGateStep(wf *parser.Workflow, stepName string) bool {
	for _, s := range wf.Steps {
		if s.Name == stepName {
			return s.Type == "gate"
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
