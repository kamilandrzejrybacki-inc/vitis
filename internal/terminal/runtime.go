package terminal

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"
	"github.com/kamilandrzejrybacki-inc/clank/internal/adapter"
	"github.com/kamilandrzejrybacki-inc/clank/internal/model"
)

type PseudoTerminalRuntime interface {
	Spawn(spec adapter.SpawnSpec) (PseudoTerminalProcess, error)
}

type PseudoTerminalProcess interface {
	Write(data []byte) (int, error)
	Output() <-chan model.StreamEvent
	Done() <-chan model.ExitResult
	Terminate(gracePeriodMs int) error
}

type Runtime struct{}

func NewRuntime() *Runtime {
	return &Runtime{}
}

func (r *Runtime) Spawn(spec adapter.SpawnSpec) (PseudoTerminalProcess, error) {
	if spec.Command == "" {
		return nil, fmt.Errorf("missing command")
	}

	cmd := exec.Command(spec.Command, spec.Args...)
	cmd.Dir = spec.Cwd

	env := os.Environ()
	if spec.HomeDir != "" {
		env = append(env, "HOME="+spec.HomeDir)
	}
	for k, v := range spec.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	size := &pty.Winsize{
		Rows: uint16(spec.TerminalRows),
		Cols: uint16(spec.TerminalCols),
	}
	if size.Rows == 0 {
		size.Rows = 24
	}
	if size.Cols == 0 {
		size.Cols = 80
	}

	ptmx, err := pty.StartWithSize(cmd, size)
	if err != nil {
		return nil, fmt.Errorf("start pty: %w", err)
	}

	return newProcess(cmd, ptmx), nil
}
