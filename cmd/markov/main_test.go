package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestDefaultStateStorePath_OutOfCluster(t *testing.T) {
	t.Setenv(stateStoreEnv, "")

	old := saTokenPath
	saTokenPath = "/nonexistent/path/token"
	defer func() { saTokenPath = old }()

	got := defaultStateStorePath()
	if got != "./markov-state.db" {
		t.Errorf("defaultStateStorePath() = %q, want ./markov-state.db", got)
	}
}

func TestDefaultStateStorePath_InCluster(t *testing.T) {
	t.Setenv(stateStoreEnv, "")

	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("fake-token"), 0644); err != nil {
		t.Fatal(err)
	}

	old := saTokenPath
	saTokenPath = tokenFile
	defer func() { saTokenPath = old }()

	got := defaultStateStorePath()
	if got != "/tmp/markov-state.db" {
		t.Errorf("defaultStateStorePath() = %q, want /tmp/markov-state.db", got)
	}
}

func TestDefaultStateStorePath_EnvironmentOverride(t *testing.T) {
	t.Setenv(stateStoreEnv, "postgres://markov:secret@postgres:5432/markov?sslmode=disable")

	tokenFile := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenFile, []byte("fake-token"), 0644); err != nil {
		t.Fatal(err)
	}

	old := saTokenPath
	saTokenPath = tokenFile
	defer func() { saTokenPath = old }()

	got := defaultStateStorePath()
	want := "postgres://markov:secret@postgres:5432/markov?sslmode=disable"
	if got != want {
		t.Errorf("defaultStateStorePath() = %q, want %q", got, want)
	}
}

func TestAddStateStoreFlagRedactsPostgresDefault(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addStateStoreFlag(cmd, "postgres://markov:secret@postgres:5432/markov?sslmode=disable")

	flag := cmd.Flags().Lookup("state-store")
	if flag == nil {
		t.Fatal("state-store flag was not registered")
	}

	want := "postgres://<redacted>@postgres:5432/markov?<redacted>"
	if flag.DefValue != want {
		t.Errorf("DefValue = %q, want %q", flag.DefValue, want)
	}
	if flag.Value.String() != "postgres://markov:secret@postgres:5432/markov?sslmode=disable" {
		t.Errorf("runtime value was not preserved")
	}
}

func TestResolveNamespace_ExplicitWorkflow(t *testing.T) {
	old := saNamespacePath
	saNamespacePath = "/nonexistent"
	defer func() { saNamespacePath = old }()

	got := resolveNamespace("prod", "staging")
	if got != "prod" {
		t.Errorf("resolveNamespace() = %q, want prod", got)
	}
}

func TestResolveNamespace_FlagFallback(t *testing.T) {
	old := saNamespacePath
	saNamespacePath = "/nonexistent"
	defer func() { saNamespacePath = old }()

	got := resolveNamespace("", "staging")
	if got != "staging" {
		t.Errorf("resolveNamespace() = %q, want staging", got)
	}
}

func TestResolveNamespace_ServiceAccountFallback(t *testing.T) {
	nsFile := filepath.Join(t.TempDir(), "namespace")
	if err := os.WriteFile(nsFile, []byte("ai-pipeline\n"), 0644); err != nil {
		t.Fatal(err)
	}

	old := saNamespacePath
	saNamespacePath = nsFile
	defer func() { saNamespacePath = old }()

	got := resolveNamespace("", "")
	if got != "ai-pipeline" {
		t.Errorf("resolveNamespace() = %q, want ai-pipeline", got)
	}
}

func TestResolveNamespace_DefaultFallback(t *testing.T) {
	old := saNamespacePath
	saNamespacePath = "/nonexistent"
	defer func() { saNamespacePath = old }()

	got := resolveNamespace("", "")
	if got != "default" {
		t.Errorf("resolveNamespace() = %q, want default", got)
	}
}
