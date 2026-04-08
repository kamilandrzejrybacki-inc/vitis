package terminal

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/kamilandrzejrybacki-inc/vitis/internal/model"
)

type process struct {
	cmd       *exec.Cmd
	ptmx      *os.File
	outputCh  chan model.StreamEvent
	doneCh    chan model.ExitResult
	closeOnce sync.Once
	termOnce  sync.Once
}

func newProcess(cmd *exec.Cmd, ptmx *os.File) *process {
	p := &process{
		cmd:      cmd,
		ptmx:     ptmx,
		outputCh: make(chan model.StreamEvent, 128),
		doneCh:   make(chan model.ExitResult, 1),
	}
	go p.readOutput()
	go p.wait()
	return p
}

func (p *process) Write(data []byte) (int, error) {
	return p.ptmx.Write(data)
}

func (p *process) Output() <-chan model.StreamEvent {
	return p.outputCh
}

func (p *process) Done() <-chan model.ExitResult {
	return p.doneCh
}

func (p *process) Terminate(gracePeriodMs int) error {
	var termErr error
	p.termOnce.Do(func() {
		if p.cmd.Process == nil {
			return
		}
		if err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGINT); err != nil && !errors.Is(err, os.ErrProcessDone) {
			termErr = err
			return
		}

		timer := time.NewTimer(time.Duration(gracePeriodMs) * time.Millisecond)
		defer timer.Stop()

		select {
		case <-p.doneCh:
			return
		case <-timer.C:
			_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	return termErr
}

func (p *process) readOutput() {
	defer p.closeOutput()

	buf := make([]byte, 4096)
	for {
		n, err := p.ptmx.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			p.outputCh <- model.StreamEvent{
				Timestamp: time.Now(),
				Kind:      model.StreamEventOutput,
				Data:      chunk,
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			return
		}
	}
}

func (p *process) wait() {
	err := p.cmd.Wait()
	// close ptmx first → causes readOutput to get EOF and exit cleanly
	_ = p.ptmx.Close()

	result := model.ExitResult{Code: 0}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.Code = exitErr.ExitCode()
		} else {
			result.Err = err
		}
	}
	p.doneCh <- result
	close(p.doneCh)
}

func (p *process) closeOutput() {
	p.closeOnce.Do(func() {
		close(p.outputCh)
	})
}
