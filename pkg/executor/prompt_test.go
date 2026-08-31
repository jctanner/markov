package executor

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestPromptReturnsInteractiveChoice(t *testing.T) {
	var output bytes.Buffer
	prompt := &Prompt{
		in:          strings.NewReader("yes\n"),
		out:         &output,
		interactive: func() bool { return true },
	}

	result, err := prompt.Execute(context.Background(), map[string]any{
		"message": "Approve release?",
		"choices": []any{"yes", "no"},
		"default": "no",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Output["value"], "yes"; got != want {
		t.Fatalf("value = %v, want %q", got, want)
	}
	if got := output.String(); got != "Approve release? [yes/no] (default: no): " {
		t.Fatalf("prompt output = %q", got)
	}
}

func TestPromptUsesDefaultForEmptyInput(t *testing.T) {
	prompt := &Prompt{
		in:          strings.NewReader("\n"),
		out:         &bytes.Buffer{},
		interactive: func() bool { return true },
	}
	result, err := prompt.Execute(context.Background(), map[string]any{
		"message": "Continue?",
		"default": "no",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Output["value"], "no"; got != want {
		t.Fatalf("value = %v, want %q", got, want)
	}
}

func TestPromptAcceptsCaseInsensitiveChoiceAndReturnsCanonicalValue(t *testing.T) {
	prompt := &Prompt{
		in:          strings.NewReader("YES\n"),
		out:         &bytes.Buffer{},
		interactive: func() bool { return true },
	}
	result, err := prompt.Execute(context.Background(), map[string]any{
		"message":          "Approve?",
		"choices":          []any{"yes", "no"},
		"case_insensitive": true,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Output["value"], "yes"; got != want {
		t.Fatalf("value = %v, want canonical %q", got, want)
	}
}

func TestPromptRepromptsForInvalidChoice(t *testing.T) {
	var output bytes.Buffer
	prompt := &Prompt{
		in:          strings.NewReader("maybe\nyes\n"),
		out:         &output,
		interactive: func() bool { return true },
	}
	result, err := prompt.Execute(context.Background(), map[string]any{
		"message": "Approve?",
		"choices": []any{"yes", "no"},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := result.Output["value"], "yes"; got != want {
		t.Fatalf("value = %v, want %q", got, want)
	}
	if got := output.String(); !strings.Contains(got, `Invalid input "maybe". Choose one of: yes, no.`) {
		t.Fatalf("prompt output = %q, want invalid-choice message", got)
	}
}

func TestPromptRejectsNonInteractiveInput(t *testing.T) {
	nonInteractive := &Prompt{
		in:          strings.NewReader("yes\n"),
		out:         &bytes.Buffer{},
		interactive: func() bool { return false },
	}
	if _, err := nonInteractive.Execute(context.Background(), map[string]any{"message": "Approve?"}); err == nil {
		t.Fatal("Execute() error = nil, want terminal error")
	}
}
