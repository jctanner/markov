package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jctanner/markov/pkg/callback"
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
	flagSteps              bool
	flagVerbose            bool
	flagCallbacks          []string
	flagCallbackHeaders    []string
	flagCallbackTLSInsecure bool
	flagCallbackTLSCert    string
	flagCallbackBufferSize int
	flagDebug              bool
	flagRunID              string

	saTokenPath     = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	saNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

func debugLog(format string, args ...any) {
	if flagDebug {
		log.Printf("[debug] "+format, args...)
	}
}

func defaultStateStorePath() string {
	if _, err := os.Stat(saTokenPath); err == nil {
		return "/tmp/markov-state.db"
	}
	return "./markov-state.db"
}

func main() {
	stateStorePath := defaultStateStorePath()

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
	runCmd.Flags().StringVar(&flagStateStore, "state-store", stateStorePath, "SQLite state store path")
	runCmd.Flags().StringVar(&flagNamespace, "namespace", "", "Override K8s namespace")
	runCmd.Flags().StringVar(&flagKubeconfig, "kubeconfig", "", "K8s config path")
	runCmd.Flags().BoolVar(&flagVerbose, "verbose", false, "Show detailed execution output")
	runCmd.Flags().BoolVar(&flagDebug, "debug", false, "Show debug logging for flag parsing, callback setup, and K8s client init")
	runCmd.Flags().StringVar(&flagRunID, "run-id", "", "Use a specific run ID instead of generating one")
	runCmd.Flags().StringArrayVar(&flagCallbacks, "callback", nil, "Callback destination URL (repeatable). Schemes: jsonl://, http://, https://, grpc://, grpcs://")
	runCmd.Flags().StringArrayVar(&flagCallbackHeaders, "callback-header", nil, "Extra HTTP headers for http callbacks (key=value, repeatable)")
	runCmd.Flags().BoolVar(&flagCallbackTLSInsecure, "callback-tls-insecure", false, "Skip TLS verification for callback connections")
	runCmd.Flags().StringVar(&flagCallbackTLSCert, "callback-tls-cert", "", "Client TLS certificate for callback connections")
	runCmd.Flags().IntVar(&flagCallbackBufferSize, "callback-buffer-size", 1000, "Async send buffer size for callbacks")

	resumeCmd := &cobra.Command{
		Use:   "resume <run_id>",
		Short: "Resume a failed workflow run",
		Args:  cobra.ExactArgs(1),
		RunE:  resumeWorkflow,
	}
	resumeCmd.Flags().StringVar(&flagStateStore, "state-store", stateStorePath, "SQLite state store path")

	statusCmd := &cobra.Command{
		Use:   "status <run_id>",
		Short: "Show run status",
		Args:  cobra.ExactArgs(1),
		RunE:  showStatus,
	}
	statusCmd.Flags().StringVar(&flagStateStore, "state-store", stateStorePath, "SQLite state store path")
	statusCmd.Flags().BoolVar(&flagSteps, "steps", false, "Show individual step statuses")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all workflow runs",
		Args:  cobra.NoArgs,
		RunE:  listRuns,
	}
	listCmd.Flags().StringVar(&flagStateStore, "state-store", stateStorePath, "SQLite state store path")

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
	diagramCmd.Flags().StringVar(&flagStateStore, "state-store", stateStorePath, "SQLite state store path")

	root.AddCommand(runCmd, resumeCmd, statusCmd, listCmd, validateCmd, diagramCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runWorkflow(cmd *cobra.Command, args []string) error {
	if flagDebug {
		flagVerbose = true
	}

	debugLog("flags: --run-id=%q --workflow=%q --namespace=%q --kubeconfig=%q --state-store=%q --forks=%d --verbose=%v",
		flagRunID, flagWorkflow, flagNamespace, flagKubeconfig, flagStateStore, flagForks, flagVerbose)
	debugLog("flags: --callback=%v --callback-header=%v --callback-tls-insecure=%v --callback-buffer-size=%d",
		flagCallbacks, flagCallbackHeaders, flagCallbackTLSInsecure, flagCallbackBufferSize)
	debugLog("flags: --var=%v", flagVars)

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

	debugLog("state store: %s", flagStateStore)
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
	eng.RunID = flagRunID

	cbs, err := buildCallbacks()
	if err != nil {
		return err
	}
	debugLog("callbacks: %d created from %d --callback flags", len(cbs), len(flagCallbacks))
	if len(cbs) > 0 {
		eng.SetCallbacks(cbs)
		defer eng.CloseCallbacks()
	}

	k8sClient, restCfg, err := getK8sClient()
	if err == nil {
		eng.SetK8sClient(k8sClient, restCfg)
	} else {
		debugLog("k8s client: unavailable: %v", err)
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

func buildCallbacks() ([]callback.Callback, error) {
	if len(flagCallbacks) == 0 {
		return nil, nil
	}

	headers := make(map[string]string)
	for _, h := range flagCallbackHeaders {
		parts := strings.SplitN(h, "=", 2)
		if len(parts) == 2 {
			headers[parts[0]] = parts[1]
		}
	}

	var cbs []callback.Callback
	for _, u := range flagCallbacks {
		debugLog("callback: parsing %q", u)
		cb, err := callback.ParseCallbackURL(u, headers, flagCallbackBufferSize, flagCallbackTLSInsecure, flagCallbackTLSCert)
		if err != nil {
			return nil, fmt.Errorf("parsing callback %q: %w", u, err)
		}
		debugLog("callback: created %T for %s", cb, u)
		cbs = append(cbs, cb)
	}
	return cbs, nil
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

func resolveNamespace(wfNamespace, flagNS string) string {
	ns := wfNamespace
	if ns != "" {
		debugLog("namespace: using workflow namespace %q", ns)
		return ns
	}
	ns = flagNS
	if ns != "" {
		debugLog("namespace: using --namespace flag %q", ns)
		return ns
	}
	if data, err := os.ReadFile(saNamespacePath); err == nil {
		ns = strings.TrimSpace(string(data))
		debugLog("namespace: using service account namespace %q", ns)
		return ns
	}
	debugLog("namespace: using default")
	return "default"
}

func buildExecutors(wf *parser.WorkflowFile) (map[string]executor.Executor, error) {
	executors := map[string]executor.Executor{
		"shell_exec":   executor.NewShellExec(),
		"http_request": executor.NewHTTPRequest(),
	}

	namespace := resolveNamespace(wf.Namespace, flagNamespace)

	k8sClient, _, err := getK8sClient()
	if err != nil {
		log.Printf("warning: k8s client unavailable: %v (k8s_job steps will fail)", err)
	} else {
		executors["k8s_job"] = executor.NewK8sJob(k8sClient, namespace)
	}

	debugLog("executors: registered %v", func() []string {
		names := make([]string, 0, len(executors))
		for k := range executors {
			names = append(names, k)
		}
		return names
	}())

	return executors, nil
}

func getK8sClient() (kubernetes.Interface, *rest.Config, error) {
	if restConfig, err := rest.InClusterConfig(); err == nil {
		debugLog("k8s client: using in-cluster config (host=%s)", restConfig.Host)
		client, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			return nil, nil, err
		}
		return client, restConfig, nil
	}
	debugLog("k8s client: in-cluster config not available, trying kubeconfig")

	kubeconfig := flagKubeconfig
	if kubeconfig == "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	var config *clientcmd.ClientConfig
	debugLog("k8s client: kubeconfig=%q", kubeconfig)
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
