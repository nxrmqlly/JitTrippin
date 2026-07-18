package engine

import (
	"context"
	"io"
)

type Executor interface {
    Run(ctx context.Context, s *Scheduler, r Runner, stdout io.Writer, stderr io.Writer)
}