package executor

import "context"

type Result struct {
	Output map[string]any
	Error  error
}

type Executor interface {
	Execute(ctx context.Context, params map[string]any) (*Result, error)
}
