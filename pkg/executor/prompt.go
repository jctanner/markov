package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Prompt reads one interactive response from the runner terminal.
// It is intentionally a host-only primitive; use gate pause/resume for
// approvals that must survive a runner process exiting.
type Prompt struct {
	in          io.Reader
	out         io.Writer
	interactive func() bool
}

func NewPrompt() *Prompt {
	return &Prompt{
		in:  os.Stdin,
		out: os.Stdout,
		interactive: func() bool {
			return term.IsTerminal(int(os.Stdin.Fd()))
		},
	}
}

func (p *Prompt) Execute(_ context.Context, params map[string]any) (*Result, error) {
	if !p.interactive() {
		return nil, fmt.Errorf("prompt: interactive terminal input is required; use gate pause/resume for non-interactive approval")
	}

	message, ok := params["message"].(string)
	if !ok || strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("prompt: message is required")
	}
	defaultValue, err := promptStringParam(params, "default")
	if err != nil {
		return nil, err
	}
	choices, err := promptChoices(params)
	if err != nil {
		return nil, err
	}
	caseInsensitive, err := promptBoolParam(params, "case_insensitive")
	if err != nil {
		return nil, err
	}
	if defaultValue != "" && len(choices) > 0 {
		canonical, ok := matchingChoice(choices, defaultValue, caseInsensitive)
		if !ok {
			return nil, fmt.Errorf("prompt: default %q is not one of the configured choices", defaultValue)
		}
		defaultValue = canonical
	}

	scanner := bufio.NewScanner(p.in)
	for {
		writePrompt(p.out, message, choices, defaultValue)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("prompt: reading input: %w", err)
			}
			return nil, fmt.Errorf("prompt: no input received")
		}
		value := strings.TrimSpace(scanner.Text())
		if value == "" {
			value = defaultValue
		}
		if value == "" {
			fmt.Fprintln(p.out, "Input is required.")
			continue
		}
		if len(choices) > 0 {
			canonical, ok := matchingChoice(choices, value, caseInsensitive)
			if ok {
				return &Result{Output: map[string]any{"value": canonical}}, nil
			}
			fmt.Fprintf(p.out, "Invalid input %q. Choose one of: %s.\n", value, strings.Join(choices, ", "))
			continue
		}

		return &Result{Output: map[string]any{"value": value}}, nil
	}
}

func writePrompt(out io.Writer, message string, choices []string, defaultValue string) {
	fmt.Fprint(out, message)
	if len(choices) > 0 {
		fmt.Fprintf(out, " [%s]", strings.Join(choices, "/"))
	}
	if defaultValue != "" {
		fmt.Fprintf(out, " (default: %s)", defaultValue)
	}
	fmt.Fprint(out, ": ")
}

func promptStringParam(params map[string]any, name string) (string, error) {
	raw, exists := params[name]
	if !exists {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("prompt: %s must be a string", name)
	}
	return value, nil
}

func promptChoices(params map[string]any) ([]string, error) {
	raw, exists := params["choices"]
	if !exists {
		return nil, nil
	}
	var choices []string
	switch values := raw.(type) {
	case []string:
		choices = values
	case []any:
		choices = make([]string, len(values))
		for i, value := range values {
			choice, ok := value.(string)
			if !ok || choice == "" {
				return nil, fmt.Errorf("prompt: choices[%d] must be a non-empty string", i)
			}
			choices[i] = choice
		}
	default:
		return nil, fmt.Errorf("prompt: choices must be a list of strings")
	}
	if len(choices) == 0 {
		return nil, fmt.Errorf("prompt: choices must not be empty")
	}
	for i, choice := range choices {
		if choice == "" {
			return nil, fmt.Errorf("prompt: choices[%d] must be a non-empty string", i)
		}
	}
	return choices, nil
}

func promptBoolParam(params map[string]any, name string) (bool, error) {
	raw, exists := params[name]
	if !exists {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("prompt: %s must be a boolean", name)
	}
	return value, nil
}

func matchingChoice(choices []string, value string, caseInsensitive bool) (string, bool) {
	for _, choice := range choices {
		if choice == value || caseInsensitive && strings.EqualFold(choice, value) {
			return choice, true
		}
	}
	return "", false
}
