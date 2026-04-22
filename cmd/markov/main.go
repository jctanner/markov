package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jctanner/markov/pkg/engine"
	"github.com/jctanner/markov/pkg/executor"
	"github.com/jctanner/markov/pkg/parser"
	"github.com/jctanner/markov/pkg/state"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	flagVars       []string
	flagWorkflow   string
	flagForks      int
	flagStateStore string
	flagNamespace  string
	flagKubeconfig string
	flagSteps      bool
	flagVerbose    bool
)

func main() {
	root := &cobra.Command{
		Use:   "markov",
		Short: "YAML workflow engine for Kubernetes",
	}

	runCmd := &cobra.Command{
		Use:   "run <file.yaml>",
		Short: "Run a workflow",
		Args:  cobra.ExactArgs(1),
		RunE:  runWorkflow,
	}
	runCmd.Flags().StringArrayVar(&flagVars, "var", nil, "Override vars (key=value, repeatable)")
	runCmd.Flags().StringVar(&flagWorkflow, "workflow", "", "Run a specific workflow instead of entrypoint")
	runCmd.Flags().IntVar(&flagForks, "forks", 0, "Override global forks")
	runCmd.Flags().StringVar(&flagStateStore, "state-store", "./markov-state.db", "SQLite state store path")
	runCmd.Flags().StringVar(&flagNamespace, "namespace", "", "Override K8s namespace")
	runCmd.Flags().StringVar(&flagKubeconfig, "kubeconfig", "", "K8s config path")
	runCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Show detailed execution output")

	resumeCmd := &cobra.Command{
		Use:   "resume <run_id>",
		Short: "Resume a failed workflow run",
		Args:  cobra.ExactArgs(1),
		RunE:  resumeWorkflow,
	}
	resumeCmd.Flags().StringVar(&flagStateStore, "state-store", "./markov-state.db", "SQLite state store path")

	statusCmd := &cobra.Command{
		Use:   "status <run_id>",
		Short: "Show run status",
		Args:  cobra.ExactArgs(1),
		RunE:  showStatus,
	}
	statusCmd.Flags().StringVar(&flagStateStore, "state-store", "./markov-state.db", "SQLite state store path")
	statusCmd.Flags().BoolVar(&flagSteps, "steps", false, "Show individual step statuses")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all workflow runs",
		Args:  cobra.NoArgs,
		RunE:  listRuns,
	}
	listCmd.Flags().StringVar(&flagStateStore, "state-store", "./markov-state.db", "SQLite state store path")

	validateCmd := &cobra.Command{
		Use:   "validate <file.yaml>",
		Short: "Validate a workflow file",
		Args:  cobra.ExactArgs(1),
		RunE:  validateWorkflow,
	}

	diagramCmd := &cobra.Command{
		Use:   "diagram <run_id>",
		Short: "Generate a Mermaid diagram of a completed run",
		Args:  cobra.ExactArgs(1),
		RunE:  showDiagram,
	}
	diagramCmd.Flags().StringVar(&flagStateStore, "state-store", "./markov-state.db", "SQLite state store path")

	root.AddCommand(runCmd, resumeCmd, statusCmd, listCmd, validateCmd, diagramCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runWorkflow(cmd *cobra.Command, args []string) error {
	wfFile, err := parser.ParseFile(args[0])
	if err != nil {
		return err
	}

	if flagForks > 0 {
		wfFile.Forks = flagForks
	}
	if flagNamespace != "" {
		wfFile.Namespace = flagNamespace
	}

	vars := parseVarFlags(flagVars)

	store, err := state.NewSQLiteStore(flagStateStore)
	if err != nil {
		return err
	}
	defer store.Close()

	executors, err := buildExecutors(wfFile)
	if err != nil {
		return err
	}

	eng := engine.New(wfFile, store, executors)
	eng.Verbose = flagVerbose

	k8sClient, restCfg, err := getK8sClient()
	if err == nil {
		eng.SetK8sClient(k8sClient, restCfg)
	}

	ctx := context.Background()
	runID, err := eng.Run(ctx, flagWorkflow, vars)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run %s failed: %v\n", runID, err)
		return err
	}

	fmt.Printf("run %s completed successfully\n", runID)
	return nil
}

func resumeWorkflow(cmd *cobra.Command, args []string) error {
	store, err := state.NewSQLiteStore(flagStateStore)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	run, err := store.GetRun(ctx, args[0])
	if err != nil {
		return err
	}

	wfFile, err := parser.ParseFile(run.WorkflowFile)
	if err != nil {
		return err
	}

	executors, err := buildExecutors(wfFile)
	if err != nil {
		return err
	}

	eng := engine.New(wfFile, store, executors)
	return eng.Resume(ctx, args[0])
}

func showStatus(cmd *cobra.Command, args []string) error {
	store, err := state.NewSQLiteStore(flagStateStore)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	run, err := store.GetRun(ctx, args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Run:        %s\n", run.RunID)
	fmt.Printf("Workflow:   %s\n", run.Entrypoint)
	fmt.Printf("Status:     %s\n", run.Status)
	fmt.Printf("Started:    %s\n", run.StartedAt.Format("2006-01-02 15:04:05"))
	if run.CompletedAt != nil {
		fmt.Printf("Completed:  %s\n", run.CompletedAt.Format("2006-01-02 15:04:05"))
	}

	if flagSteps {
		steps, err := store.GetSteps(ctx, args[0])
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Printf("%-30s %-12s %s\n", "STEP", "STATUS", "DURATION")
		fmt.Printf("%-30s %-12s %s\n", "----", "------", "--------")
		for _, s := range steps {
			dur := ""
			if s.StartedAt != nil && s.CompletedAt != nil {
				dur = s.CompletedAt.Sub(*s.StartedAt).Round(100 * 1e6).String()
			}
			fmt.Printf("%-30s %-12s %s\n", s.StepName, s.Status, dur)
			if s.Error != "" {
				fmt.Printf("  error: %s\n", s.Error)
			}
		}
	}

	return nil
}

func listRuns(cmd *cobra.Command, args []string) error {
	store, err := state.NewSQLiteStore(flagStateStore)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	runs, err := store.ListRuns(ctx)
	if err != nil {
		return err
	}

	if len(runs) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	fmt.Printf("%-10s %-25s %-12s %-20s %s\n", "RUN ID", "WORKFLOW", "STATUS", "STARTED", "DURATION")
	fmt.Printf("%-10s %-25s %-12s %-20s %s\n", "------", "--------", "------", "-------", "--------")
	for _, r := range runs {
		dur := ""
		if r.CompletedAt != nil {
			dur = r.CompletedAt.Sub(r.StartedAt).Round(100 * 1e6).String()
		}
		fmt.Printf("%-10s %-25s %-12s %-20s %s\n",
			r.RunID, r.Entrypoint, r.Status,
			r.StartedAt.Format("2006-01-02 15:04:05"), dur)
	}

	return nil
}

func showDiagram(cmd *cobra.Command, args []string) error {
	store, err := state.NewSQLiteStore(flagStateStore)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	tree, err := buildRunTree(ctx, store, args[0])
	if err != nil {
		return err
	}

	fmt.Print(generateMermaid(tree))
	return nil
}

func validateWorkflow(cmd *cobra.Command, args []string) error {
	_, err := parser.ParseFile(args[0])
	if err != nil {
		return err
	}
	fmt.Println("valid")
	return nil
}

func parseVarFlags(vars []string) map[string]any {
	result := make(map[string]any)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func buildExecutors(wf *parser.WorkflowFile) (map[string]executor.Executor, error) {
	executors := map[string]executor.Executor{
		"shell_exec":   executor.NewShellExec(),
		"http_request": executor.NewHTTPRequest(),
	}

	namespace := wf.Namespace
	if namespace == "" {
		namespace = "default"
	}

	k8sClient, _, err := getK8sClient()
	if err != nil {
		log.Printf("warning: k8s client unavailable: %v (k8s_job steps will fail)", err)
	} else {
		executors["k8s_job"] = executor.NewK8sJob(k8sClient, namespace)
	}

	return executors, nil
}

func getK8sClient() (kubernetes.Interface, *rest.Config, error) {
	kubeconfig := flagKubeconfig
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	var config *clientcmd.ClientConfig
	if kubeconfig != "" {
		c := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
			&clientcmd.ConfigOverrides{},
		)
		config = &c
	} else {
		c := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			clientcmd.NewDefaultClientConfigLoadingRules(),
			&clientcmd.ConfigOverrides{},
		)
		config = &c
	}

	restConfig, err := (*config).ClientConfig()
	if err != nil {
		return nil, nil, err
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	return client, restConfig, nil
}
