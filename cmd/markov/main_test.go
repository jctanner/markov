package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultStateStorePath_OutOfCluster(t *testing.T) {
	old := saTokenPath
	saTokenPath = "/nonexistent/path/token"
	defer func() { saTokenPath = old }()

	got := defaultStateStorePath()
	if got != "./markov-state.db" {
		t.Errorf("defaultStateStorePath() = %q, want ./markov-state.db", got)
	}
}

func TestDefaultStateStorePath_InCluster(t *testing.T) {
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
